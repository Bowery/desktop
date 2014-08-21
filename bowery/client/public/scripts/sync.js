$(document).ready(function () {

  if (!window['WebSocket']) {
    return
  }
  var conn = new WebSocket('ws://' + window.location.host + '/_/ws')
  var $syncStatusEl = $('.status-description')

  function upToDate() {
    $syncStatusEl.text('Up to date.')
    $('.pause, .check, .play')
      .removeClass('pause')
      .removeClass('play')
      .addClass('check')
  }

  $('.status-icon').click(function (e) {
    e.preventDefault()

    if ($(e.target).hasClass('pause') || $(e.target).hasClass('check') ) {
      $.get('/pause', function () {
        $(e.target).removeClass('pause')
                  .removeClass('check')
                  .addClass('play')
        $syncStatusEl.text('Syncing paused.')
      })
      return
    }

    if ($(e.target).hasClass('play')) {
        $.get('/resume', function () {
          upToDate()
        })
    }
  })


  conn.onopen = function (ev) {
    console.log('open', ev)
  }

  conn.onclose = function (ev) {
    console.log('close', ev)
  }

  conn.onmessage = function (ev) {
    var data = JSON.parse(ev.data)
    var appID = data.application.id
    var $appEl = $('.item[data-app="' + appID + '"]')

    console.log(data)

    // Handle errors.
    if (data.error) {
      $appEl.find('.status').addClass('alert')
      $syncStatusEl.text("Error: " + data.error + ".")
      setTimeout(upToDate, 750)
    }

    // Check for connect/disconnect status.
    if (data.status == 'connect')
      $appEl.find('.status').removeClass('alert')
    if (data.status == 'disconnect')
      $appEl.find('.status').addClass('alert')

    if (data.status == 'update') {
      $appEl.find('.status').removeClass('alert')
      $syncStatusEl.text('Updated ' + data.path + ".")
      setTimeout(upToDate, 750)
    }

    if (data.status == 'create') {
      $appEl.find('.status').removeClass('alert')
      $syncStatusEl.text('Created ' + data.path + ".")
      setTimeout(upToDate, 750)
    }

    if (data.status == 'delete') {
      $appEl.find('.status').removeClass('alert')
      $syncStatusEl.text('Deleted ' + data.path + ".")
      setTimeout(upToDate, 750)
    }

    if (data.status == 'upload-start')
      $syncStatusEl.text('Uploading ' + (data.application.name || data.application.localPath) + ".")
    if (data.status == 'upload-finish')
      $syncStatusEl.text('Up to date.')

  }

  $('.toggle').click(function (e) {
    e.preventDefault()
    var target
    if ($(e.target).hasClass("toggle")) {
      target = $(e.target)
    } else {
      target = $(e.target).parent()
    }

    target.toggleClass("active")

    if (target.closest('.group.plugin').length > 0) {
      $.ajax({
        method: "POST",
        url: "/applications/" + target.data("app") + "/plugins/" + target.data("plugin-name") + "/" + target.data("plugin-version"),
        data: {
          plugin: target.data("version"),
          app: target.data("app")
        }
      })
      .done(function () {
        console.log(arguments)
      })
    } else if (target.closest('.group.settings').length > 0) {
      $.ajax({
        method: 'POST',
        url: '/settings/dev-mode'
      })
      .done(function() {
        console.log(arguments)
      })
    }
  })
})
