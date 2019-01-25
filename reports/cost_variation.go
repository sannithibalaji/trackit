//   Copyright 2018 MSolution.IO
//
//   Licensed under the Apache License, Version 2.0 (the "License");
//   you may not use this file except in compliance with the License.
//   You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
//   Unless required by applicable law or agreed to in writing, software
//   distributed under the License is distributed on an "AS IS" BASIS,
//   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//   See the License for the specific language governing permissions and
//   limitations under the License.

package reports

import (
"context"
"database/sql"
"fmt"
"strconv"
"strings"
"time"

"github.com/360EntSecGroup-Skylar/excelize"
"github.com/trackit/jsonlog"

"github.com/trackit/trackit-server/aws"
"github.com/trackit/trackit-server/aws/usageReports/history"
"github.com/trackit/trackit-server/costs/diff"
)

type costVariationProduct map[time.Time]diff.PricePoint
type costVariationReport map[string]costVariationProduct

const (
	freqDaily = iota
	freqMonthly
)

const costVariationLastMonthSheetName = "Cost Variations (Last Month)"
const costVariationLast6MonthsSheetName = "Cost Variations (Last 6 Months)"

var costVariationLastMonth = module {
	Name:          "Cost Variations (Last Month)",
	SheetName:     costVariationLastMonthSheetName,
	ErrorName:     "costVariationLastMonthError",
	GenerateSheet: costVariationGenerateLastMonth,
}

var costVariationLast6Months = module {
	Name:          "Cost Variations (Last 6 Months)",
	SheetName:     costVariationLast6MonthsSheetName,
	ErrorName:     "costVariationLastSixMonthsError",
	GenerateSheet: costVariationGenerateLast6Months,
}

func costVariationGenerateLastMonth(ctx context.Context, aas []aws.AwsAccount, date time.Time, _ *sql.Tx, file *excelize.File) (err error) {
	var dateRange = make([]time.Time, 2)
	if date.IsZero() {
		dateRange[0], dateRange[1] = history.GetHistoryDate()
	} else {
		dateRange[0] = date
		dateRange[1] = time.Date(date.Year(), date.Month()+1, 0, 23, 59, 59, 999999999, date.Location()).UTC()
	}
	return costVariationGenerateSheet(ctx, aas, dateRange, freqDaily, file)
}

func costVariationGenerateLast6Months(ctx context.Context, aas []aws.AwsAccount, date time.Time, _ *sql.Tx, file *excelize.File) (err error) {
	var dateRange = make([]time.Time, 2)
	if date.IsZero() {
		_, dateRange[1] = history.GetHistoryDate()
	} else {
		dateRange[1] = time.Date(date.Year(), date.Month() + 1, 0, 23, 59, 59, 999999999, date.Location()).UTC()
	}
	dateRange[0] = time.Date(dateRange[1].Year(), dateRange[1].Month() - 5, 1, 0, 0, 0, 0, dateRange[1].Location()).UTC()

	return costVariationGenerateSheet(ctx, aas, dateRange, freqMonthly, file)
}

func costVariationGenerateSheet(ctx context.Context, aas []aws.AwsAccount, dateRange []time.Time, frequency int, file *excelize.File) (err error) {
	data, err := costVariationGetData(ctx, aas, dateRange, frequency)
	if err == nil {
		return costVariationInsertDataInSheet(ctx, aas, dateRange, frequency, file, data)
	}
	return
}

func costVariationGetData(ctx context.Context, aas []aws.AwsAccount, dateRange []time.Time, frequency int) (data map[aws.AwsAccount]costVariationReport, err error){
	logger := jsonlog.LoggerFromContextOrDefault(ctx)
	var aggregation string
	if frequency == freqMonthly {
		aggregation = "month"
	} else {
		aggregation = "day"
	}
	logger.Debug("Getting Cost Variation Report for accounts", map[string]interface{}{
		"accounts": aas,
		"dateStart": dateRange[0],
		"dateEnd": dateRange[1],
		"aggregation": aggregation,
	})
	data = make(map[aws.AwsAccount]costVariationReport, len(aas))
	for _, account := range aas {
		report, err := diff.TaskDiffData(ctx, account, dateRange, aggregation)
		if err != nil {
			logger.Error("An error occurred while generating a Cost Variation Report", map[string]interface{}{
				"error":     err,
				"account":   account,
				"dateStart": dateRange[0],
				"dateEnd":   dateRange[1],
			})
			return data, err
		}
		data[account] = make(costVariationReport, len(report))
		for product, values := range report {
			data[account][product], err = costVariationFormatCostDiff(values)
			if err != nil {
				logger.Error("An error occurred while parsing timestamp", map[string]interface{}{
					"error":     err,
					"account":   account,
					"values":    values,
					"dateStart": dateRange[0],
					"dateEnd":   dateRange[1],
				})
				return data, err
			}
		}
	}
	return
}

func costVariationInsertDataInSheet(ctx context.Context, aas []aws.AwsAccount, dateRange []time.Time, frequency int, file *excelize.File, data map[aws.AwsAccount]costVariationReport) (err error){
	logger := jsonlog.LoggerFromContextOrDefault(ctx)
	var sheetName string
	dates := make([]time.Time, 0)
	if frequency == freqMonthly {
		sheetName = costVariationLast6MonthsSheetName
		for index := 0; index < 6; index++ {
			dates = append(dates, dateRange[0].AddDate(0, index, 0))
		}
	} else {
		sheetName = costVariationLastMonthSheetName
		for index := 0; index < dateRange[1].Day(); index++ {
			dates = append(dates, dateRange[0].AddDate(0, 0, index))
		}
	}
	file.NewSheet(sheetName)
	costVariationGenerateHeader(file, sheetName, dates, frequency)
	line := 4
	for account, report := range data {
		for product, values := range report {
			cells := make(cells, 0, len(dates) * 2 + 2)
			cells = append(cells, newCell(formatAwsAccount(account), "A" + strconv.Itoa(line)),
									newCell(product, "B" + strconv.Itoa(line)))
			totalNeededCols := make([]string, 0, len(dates))
			for index, date := range dates {
				value := costVariationGetValueForDate(values, date)
				if index == 0 {
					cells = append(cells, newCell(value.Cost, "C" + strconv.Itoa(line)).addStyles("price"))
				} else {
					costPos := excelize.ToAlphaString(index * 2 + 2) + strconv.Itoa(line)
					previousCol := excelize.ToAlphaString(index * 2) + strconv.Itoa(line)
					posVariation := excelize.ToAlphaString(index * 2 + 1) + strconv.Itoa(line)
					formula := fmt.Sprintf(`IF(%s=0,"",%s/%s-1)`, previousCol, costPos, previousCol)
					variation := newFormula(formula, posVariation).addStyles("percentage")
					variation = variation.addConditionalFormat("negative", "green", "borders")
					variation = variation.addConditionalFormat("positive", "red", "borders")
					variation = variation.addConditionalFormat("zero", "red", "borders")
					cells = append(cells, newCell(value.Cost, costPos).addStyles("price"), variation)
					totalNeededCols = append(totalNeededCols, costPos)
				}
			}
			totalCol := len(dates) * 2 + 1
			formula := fmt.Sprintf("SUM(%s)", strings.Join(totalNeededCols, ","))
			cells = append(cells, newFormula(formula, excelize.ToAlphaString(totalCol) + strconv.Itoa(line)).addStyles("price"))
			cells.addStyles("borders", "centerText").setValues(file, sheetName)
			line++
		}
	}
	if err != nil {
		logger.Error("Error while adding Cost Variation data into spreadsheet", map[string]interface{}{
			"error":     err,
			"accounts":  aas,
			"dateStart": dateRange[0],
			"dateEnd":   dateRange[1],
		})
	}
	return
}

func costVariationGenerateHeader(file *excelize.File, sheetName string, dates []time.Time, frequency int) {
	title := "Daily Cost"
	if frequency == freqMonthly {
		title = "Monthly Cost"
	}
	header := make(cells, 0, len(dates) * 3 + 2)
	totalCol := excelize.ToAlphaString(len(dates) * 2 + 1)
	header = append(header, newCell("Account", "A1").mergeTo("A3"),
							newCell("Usage type", "B1").mergeTo("B3"),
							newCell(title, "C1").mergeTo(excelize.ToAlphaString(len(dates) * 2) + "1"),
							newCell("Total", totalCol + "1").mergeTo(totalCol + "3"))
	for index, date := range dates {
		if index == 0 {
			header = append(header, newCell(costVariationFormatDate(date, frequency), "C2"),
									newCell("Cost", "C3"))
		} else {
			col1 := excelize.ToAlphaString(index * 2 + 1)
			col2 := excelize.ToAlphaString(index * 2 + 2)
			header = append(header, newCell(costVariationFormatDate(date, frequency), col1 + "2").mergeTo(col2 + "2"),
									newCell("Variation", col1 + "3"),
									newCell("Cost", col2 + "3"))
		}
	}
	header.addStyles("borders", "bold", "centerText").setValues(file, sheetName)
	columns := columnsWidth{
		newColumnWidth("A", 30),
		newColumnWidth("B", 35),
		newColumnWidth("C", 12.5).toColumn(excelize.ToAlphaString(len(dates) * 2)),
		newColumnWidth(totalCol, 15),
	}
	columns.setValues(file, sheetName)
	return
}

func costVariationFormatDate(date time.Time, frequency int) string {
	layout := "2006-01-02"
	if frequency == freqMonthly {
		layout = "2006-01"
	}
	return date.Format(layout)
}

func costVariationFormatCostDiff(data []diff.PricePoint) (values costVariationProduct, err error) {
	values = make(costVariationProduct)
	for _, value := range data {
		date, err := time.Parse("2006-01-02T15:04:05.000Z", value.Date)
		if err != nil {
			return values, err
		}
		values[date] = value
	}
	return
}

func costVariationGetValueForDate(values costVariationProduct, date time.Time) *diff.PricePoint {
	if value, ok := values[date] ; ok {
		return &value
	}
	return nil
}