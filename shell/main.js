// Copyright 2014 Bowery, Inc.

var app = require('app')  // Module to control application life.
var BrowserWindow = require('browser-window')  // Module to create native browser window.

// Report crashes to our server.
require('crash-reporter').start()

var mainWindow = null

app.on('window-all-closed', function() {
  if (process.platform != 'darwin')
    app.quit()
})

app.on('ready', function() {
  mainWindow = new BrowserWindow({
    title: 'Bowery',
    frame: true,
    width: 350,
    height: 372,
    'node-integration': 'disable',
    resizable: false,
    center: true,
    show: false
  })

  mainWindow.show()
  mainWindow.loadUrl('http://0.0.0.0:32055/')

  mainWindow.on('closed', function() {
    mainWindow = null
  })
})

// Start Client Binary
var clientPath = require('path').join(__dirname, "../bin/client")
console.log("Launching", clientPath)
var clientProcess = require('child_process').spawn(clientPath, [])

clientProcess.on('close', function (code) {
  console.log('client process exited with code:', code)
  process.exit(code)
})
clientProcess.stdout.on('data', function (data) {
  process.stdout.write(data)
})
clientProcess.stderr.on('data', function (data) {
  process.stderr.write(data)
})
