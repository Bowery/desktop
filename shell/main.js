// Copyright 2014 Bowery, Inc.

var fs = require('fs')
var app = require('app')  // Module to control application life.
var autoUpdater = require('auto-updater')
var path = require('path')
var BrowserWindow = require('browser-window')  // Module to create native browser window.
var Menu = require('menu')

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

// Start the auto updater.
var arch = process.arch
if (arch == 'ia32') {
  arch = '386'
} else if (arch == 'x64') {
  arch = 'amd64'
}
var os = process.platform
if (os == 'win32') {
  os = 'windows'
}
var query = 'version='+app.getVersion()+'&arch='+arch+'&os='+os
autoUpdater.setFeedUrl('http://kenmare.io/client/check?'+query)

autoUpdater.on('checking-for-update', function () {
})

autoUpdater.on('update-available', function () {
})

autoUpdater.on('update-not-available', function () {
})

autoUpdater.on('update-downloaded', function (ev, notes, name, date, url, install) {
  // Display here and ask if they'd like to install the update.

  install()
})

autoUpdater.on('error', function (ev, err) {
})

// Check every 15 minutes.
autoUpdater.checkForUpdates()
setInterval(function () {
  autoUpdater.checkForUpdates()
}, 900000)

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
  var menu = Menu.buildFromTemplate(template)
  Menu.setApplicationMenu(menu)

  mainWindow = new BrowserWindow({
    title: 'Bowery',
    frame: true,
    width: 400,
    height: 485,
    resizable: false,
    center: true,
    show: true,
    frame: false,
    icon: path.join(__dirname, 'icon.png')
  })

  mainWindow.setSize(400, 485)

  mainWindow.loadUrl('http://localhost:32055/bowery/bowery.html')

  mainWindow.on('closed', function() {
    mainWindow = null
  })

  mainWindow.webContents.on('did-finish-load', function () {
    mainWindow.focus()
  })
})
