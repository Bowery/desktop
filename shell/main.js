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
    width: 650,
    height: 530,
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

// Start Client and Agent
var extension = /^win/.test(process.platform) ? ".exe" : ""

!["client", "agent"].forEach(function (binary) {
  var path = require('path').join(__dirname, "../bin/", binary + extension)
  var proc = require('child_process').spawn(path, [])

  console.log("Launching", path)

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
})
