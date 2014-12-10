// Copyright 2014 Bowery, Inc.
var fs = require('fs')
var os = require('os')
var path = require('path')
var spawn = require('child_process').spawn
var tmpdir = os.tmpdir()

// Atom shell modules.
var app = require('app')
var BrowserWindow = require('browser-window')
var Menu = require('menu')

// Set vars that control updater/client.
var versionUrl = 'http://desktop.bowery.io.s3.amazonaws.com/VERSION'
var ext = /^win/.test(process.platform) ? '.exe' : ''
var installDir = '..'
var binPath = path.join(__dirname, '..', 'bin')
var clientPath = path.join(binPath, 'client' + ext)
var updaterPath = path.join(binPath, 'updater' + ext)
var proc = null

require('crash-reporter').start() // Report crashes to our server.

// killClient kills any running updater/client process.
// NOTE: Sync ops only, the process exit event doesn't allow async ops.
var killClient = function () {
  // Kill the running instance.
  if (proc) {
    console.log('killing process started in this instance')
    proc.kill('SIGINT')
  }

  // Kill any stored pids from the updater.
  try {
    var contents = fs.readFileSync(path.join(tmpdir, 'bowery_pids'), 'utf8')
    if (contents) {
      contents = contents.split('\n')
      for (var i in contents) {
        try {
          process.kill(parseInt(contents[i], 10), 'SIGINT')
        } catch (e) {}
      }
    }
  } catch (e) {
    if (e.code != 'ENOENT') console.log("Couldn't remove previous pids", e)
  }

  // Kill the pid stored by this script.
  try {
    var pid = fs.readFileSync(path.join(tmpdir, 'bowery_client_pid'), 'utf8')
    if (pid) process.kill(parseInt(pid, 10), 'SIGINT')
  } catch (e) {
    if (e.code != 'ENOENT' && e.code != 'ESRCH') console.log("Couldn't remove previous pids", e)
  }

  try {
    fs.unlinkSync(path.join(tmpdir, 'bowery_pids'))
    fs.unlinkSync(path.join(tmpdir, 'bowery_client_pid'))
  } catch (e) {}
}

killClient()

if (process.platform == 'darwin') {
  proc = spawn(clientPath, [])
} else {
  proc = spawn(updaterPath, ["-d", installDir, versionUrl, "", clientPath])
}
proc.on('close', function (code) {
  console.log('client process exited with code:', code)
  process.exit(code)
})
proc.stdout.pipe(process.stdout)
proc.stderr.pipe(process.stderr)

// Write bowery processes info.
try {
  fs.writeFileSync(path.join(tmpdir, 'bowery_client_pid'), proc.pid)
  fs.writeFileSync(path.join(tmpdir, 'bowery_dir'), __dirname)
} catch (e) {
  console.log(e, 'could not write process info')
}

// Kill the client when we get a signal.
var exitEvents = ['SIGINT', 'SIGTERM', 'SIGHUP', 'exit', 'kill']
for (var i = 0, e; e = exitEvents[i]; i++) {
  process.on(e, function () {
    killClient()
    process.exit(0)
  })
}

app.on('window-all-closed', function() {
  app.quit()
})

app.on('ready', function() {
  // Set the menu items.
  var template = [
    {
      label: 'Bowery',
      submenu:[
        {label: 'About Bowery', selector: 'orderFrontStandardAboutPanel:'},
        {type: 'separator'},
        // Don't use CommandOrControl here, since these aren't typical on PCs.
        {label: 'Hide', accelerator: 'Command+H', selector: 'hide:'},
        {label: 'Quit', accelerator: 'Command+Q', click: function() {app.quit()}}
      ]
    },
    {
      label: 'Edit',
      submenu: [
        {
          label: 'Undo',
          accelerator: 'CommandOrControl+Z',
          selector: 'undo:'
        },
        {
          label: 'Redo',
          accelerator: 'Shift+CommandOrControl+Z',
          selector: 'redo:'
        },
        {
          type: 'separator'
        },
        {
          label: 'Cut',
          accelerator: 'CommandOrControl+X',
          selector: 'cut:'
        },
        {
          label: 'Copy',
          accelerator: 'CommandOrControl+C',
          selector: 'copy:'
        },
        {
          label: 'Paste',
          accelerator: 'CommandOrControl+V',
          selector: 'paste:'
        },
        {
          label: 'Select All',
          accelerator: 'CommandOrControl+A',
          selector: 'selectAll:'
        },
      ]
    },
    {
      label: 'View',
      submenu: [
        {
          label: 'Reload',
          accelerator: 'CommandOrControl+R',
          click: function() { BrowserWindow.getFocusedWindow().reloadIgnoringCache(); }
        },
        {
          label: 'Toggle DevTools',
          accelerator: 'Alt+CommandOrControl+I',
          click: function() { BrowserWindow.getFocusedWindow().toggleDevTools(); }
        },
      ]
    },
    {
      label: 'Window',
      submenu: [
        {
          label: 'Minimize',
          accelerator: 'CommandOrControl+M',
          selector: 'performMiniaturize:'
        },
        {
          label: 'Close',
          // Don't use CommandOrControl here, since these aren't typical on PCs.
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

  var mainWindow = new BrowserWindow({
    title: 'Bowery',
    frame: true,
    width: 400,
    height: 485,
    resizable: false,
    center: true,
    show: true,
    icon: path.join(__dirname, 'icon.png')
  })

  mainWindow.setSize(400, 485)
  mainWindow.loadUrl('file://' + path.join(__dirname, 'bowery.html'))
  mainWindow.on('closed', function () {
    mainWindow = null
  })

  mainWindow.webContents.on('did-finish-load', function () {
    mainWindow.focus()
  })
})
