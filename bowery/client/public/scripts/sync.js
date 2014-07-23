$(document).ready(function () {
  if (!window['WebSocket']) {
    return
  }
  var conn = new WebSocket('ws://localhost:3001/_/ws')

  conn.onopen = function (ev) {
    console.log("open", ev)
  }

  conn.onclose = function (ev) {
    console.log("close", ev)
  }

  conn.onmessage = function (ev) {
    var data = JSON.parse(ev.data)
    var appID = data.application.ID
    var $appEl = $('.item[data-app="' + appID + '"]')

    // Check for connect/disconnect status.
    if (data.status == 'connect')
      $appEl.find('.status').removeClass('alert')
    if (data.status == 'disconnect')
      $appEl.find('.status').addClass('alert')
  }
})
