// Copyright 2014 Bowery, Inc.
var path = require('path')

// Atom shell modules.
var app = require('app')
var BrowserWindow = require('browser-window')
var Menu = require('menu')
var exitEvents = ['SIGINT', 'SIGTERM', 'SIGHUP', 'exit', 'kill']
var clientAddr = "http://localhost:32055"
var procHandler = require(path.join(__dirname, 'proc'))
procHandler.clientAddr = clientAddr

var template = [
  {
    label: 'Bowery',
    submenu:[
      {label: 'About Bowery', selector: 'orderFrontStandardAboutPanel:'},
      {type: 'separator'},
      {label: 'Hide', accelerator: 'Command+H', selector: 'hide:'},
      {label: 'Quit', accelerator: 'Command+Q', click: function() {app.quit()}}
    ]
  },
  {
    label: 'Edit',
    submenu: [
      {
        label: 'Undo',
        accelerator: 'Command+Z',
        selector: 'undo:'
      },
      {
        label: 'Redo',
        accelerator: 'Shift+Command+Z',
        selector: 'redo:'
      },
      {
        type: 'separator'
      },
      {
        label: 'Cut',
        accelerator: 'Command+X',
        selector: 'cut:'
      },
      {
        label: 'Copy',
        accelerator: 'Command+C',
        selector: 'copy:'
      },
      {
        label: 'Paste',
        accelerator: 'Command+V',
        selector: 'paste:'
      },
      {
        label: 'Select All',
        accelerator: 'Command+A',
        selector: 'selectAll:'
      },
    ]
  },
  {
    label: 'View',
    submenu: [
      {
        label: 'Reload',
        accelerator: 'Command+R',
        click: function() { BrowserWindow.getFocusedWindow().reloadIgnoringCache(); }
      },
      {
        label: 'Toggle DevTools',
        accelerator: 'Alt+Command+I',
        click: function() { BrowserWindow.getFocusedWindow().toggleDevTools(); }
      },
    ]
  },
  {
    label: 'Window',
    submenu: [
      {
        label: 'Minimize',
        accelerator: 'Command+M',
        selector: 'performMiniaturize:'
      },
      {
        label: 'Close',
        accelerator: 'Command+W',
        selector: 'performClose:'
      },
      {
        type: 'separator'
      },
      {
        label: 'Bring All to Front',
        selector: 'arrangeInFront:'
      },
    ]
  }
]

require('crash-reporter').start() // Report crashes to our server.

app.on('ready', function() {
  // Set the menu items.
  var menu = Menu.buildFromTemplate(template)
  Menu.setApplicationMenu(menu)

  var mainWindow = new BrowserWindow({
    title: 'Bowery',
    frame: true,
    width: 400,
    height: 485,
    resizable: false,
    center: true,
    show: false,
    frame: false,
    icon: path.join(__dirname, 'icon.png')
  })
  mainWindow.setSize(400, 485)

  procHandler.startClient(function (proc) {
    mainWindow.loadUrl(clientAddr + '/bowery/bowery.html')
    mainWindow.show()

    // Kill the client when we get a signal.
    for (var i = 0, e; e = exitEvents[i]; i++) {
      process.on(e, function () {
        proc && proc.kill('SIGINT')
      })
    }

    app.on('window-all-closed', function() {
      // if (process.platform != 'darwin')
      app.quit()
      proc && proc.kill('SIGINT')
    })
  })

  mainWindow.on('closed', function() {
    mainWindow = null
  })

  mainWindow.webContents.on('did-finish-load', function () {
    mainWindow.focus()
  })
})
