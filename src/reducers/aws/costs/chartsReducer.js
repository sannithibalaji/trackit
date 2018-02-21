import Constants from '../../../constants';

export default (state={}, action) => {
  let charts = Object.assign({}, state);
  switch (action.type) {
    case Constants.AWS_INSERT_CHARTS:
      return action.charts;
    case Constants.AWS_ADD_CHART:
      charts[action.id] = action.chartType;
      return charts;
    case Constants.AWS_REMOVE_CHART:
      if (charts.hasOwnProperty(action.id))
        delete charts[action.id];
      return charts;
    default:
      return state;
  }
};
