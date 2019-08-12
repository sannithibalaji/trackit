//   Copyright 2019 MSolution.IO
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

package medialive

import (
	"context"
	"time"

	"github.com/trackit/jsonlog"

	"github.com/trackit/trackit/es"
)

const TypeMediaLiveReport = "medialive-channel-report"
const IndexPrefixMediaLiveReport = "medialive-channel-reports"
const TemplateNameMediaLiveReport = "medialive-channel-reports"

// put the ElasticSearch index for *-medialive-channel-reports indices at startup.
func init() {
	ctx, ctxCancel := context.WithTimeout(context.Background(), 10*time.Second)
	res, err := es.Client.IndexPutTemplate(TemplateNameMediaLiveReport).BodyString(TemplateMediaLiveChannelReport).Do(ctx)
	if err != nil {
		jsonlog.DefaultLogger.Error("Failed to put ES index MediaLiveReport.", err)
	} else {
		jsonlog.DefaultLogger.Info("Put ES index MediaLiveReport.", res)
		ctxCancel()
	}
}

const TemplateMediaLiveChannelReport = `
{
	"template": "*-medialive-channel-reports",
	"version": 1,
	"mappings": {
		"medialive-channel-report": {
			"properties": {
				"account": {
					"type": "keyword"
				},
				"reportDate": {
					"type": "date"
				},
				"reportType": {
					"type": "keyword"
				},
				"channel": {
					"properties": {
						"arn": {
							"type": "keyword"
						},
						"id": {
							"type": "keyword"
						},
						"name": {
							"type": "keyword"
						},
						"region": {
							"type": "boolean"
						},
						"tags": {
							"type": "nested",
							"properties": {
								"key": {
									"type": "keyword"
								},
								"value": {
									"type": "keyword"
								}
							}
						},						
						"cost": {
							"type": "nested",
							"properties": {
								"key": {
									"type": "date"
								},
								"value": {
									"type": "double"
								}
							}
						}
					}
				}
			},
			"_all": {
				"enabled": false
			},
			"numeric_detection": false,
			"date_detection": false
		}
	}
}
`
