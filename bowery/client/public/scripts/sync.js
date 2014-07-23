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
    console.log("message", ev)
  }
})
