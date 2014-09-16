// Copyright 2014 Bowery, Inc.

var fs = require('fs')
var app = require('app')  // Module to control application life.
var path = require('path')
var BrowserWindow = require('browser-window')  // Module to create native browser window.

// Report crashes to our server.
require('crash-reporter').start()
try {
  var pid = require('fs').readFileSync('/tmp/bowery_client_pid')
  if (pid) process.kill(pid)
} catch (e) {
  console.log('yolo. no pid file found.')
}


var mainWindow = null

app.on('window-all-closed', function() {
  // if (process.platform != 'darwin')
  app.quit()
  proc.kill()
})

// Start Client and Agent
var extension = /^win/.test(process.platform) ? ".exe" : ""
var clientPath = path.join(__dirname, "../bin/", "client" + extension)
var proc = require('child_process').spawn(clientPath, [])

try {
  require('fs').writeFileSync('/tmp/bowery_client_pid', proc.pid)
  require('fs').writeFileSync('/tmp/bowery_dir', __dirname)
} catch (e) {
  console.log(e, 'could not write pidfile')
}


proc.on('close', function (code) {
  console.log('client process exited with code:', code)
  process.exit(code)
})
proc.stdout.on('data', function (data) {
  process.stdout.write(data)
})
proc.stderr.on('data', function (data) {
  process.stderr.write(data)
})

// 'SIGKILL', 'SIGTERM',
var exitEvents = ['SIGINT', 'SIGTERM', 'SIGHUP', 'exit', 'kill']
for (var i = 0, e; e = exitEvents[i]; i++)
  process.on(e, function () {
      proc.kill()
  })

app.on('ready', function() {
  mainWindow = new BrowserWindow({
    title: 'Bowery',
    frame: true,
    width: 400,
    height: 460,
    resizable: false,
    center: true,
    show: true,
    frame: false,
    icon: path.join(__dirname, 'icon.png')
  })

  mainWindow.loadUrl('http://localhost:32055/bowery/bowery.html')

  mainWindow.on('closed', function() {
    mainWindow = null
  })

  mainWindow.webContents.on('did-finish-load', function () {
    mainWindow.focus()
  })
})
