// Copyright 2014 Bowery, Inc.

// Start Client Binary
var clientPath = "/usr/local/bin/bowery-client"
var clientProcess = require('child_process').spawn(clientPath, [])

clientProcess.on('close', function (code) {
  console.log('client process exited with code:', code)
  process.exit(code)
})

var app = require('app');  // Module to control application life.
var BrowserWindow = require('browser-window');  // Module to create native browser window.

// Report crashes to our server.
require('crash-reporter').start();

// Keep a global reference of the window object, if you don't, the window will
// be closed automatically when the javascript object is GCed.
var mainWindow = null;

// Quit when all windows are closed.
app.on('window-all-closed', function() {
  if (process.platform != 'darwin')
    app.quit();
});

// This method will be called when atom-shell has done everything
// initialization and ready for creating browser windows.
app.on('ready', function() {
  // Create the browser window.
  mainWindow = new BrowserWindow({width: 350, height: 350});

  mainWindow.loadUrl('http://0.0.0.0:32005')

  // and load the index.html of the app.
  // mainWindow.loadUrl('file://' + __dirname + '/index.html');

  // Emitted when the window is closed.
  mainWindow.on('closed', function() {
    // Dereference the window object, usually you would store windows
    // in an array if your app supports multi windows, this is the time
    // when you should delete the corresponding element.
    mainWindow = null;
  });
});
