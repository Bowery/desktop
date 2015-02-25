// Copyright 2014 Bowery, Inc.
var fs = require('fs')
var os = require('os')
var path = require('path')
var http = require('http')
var exec = require('child_process').exec
var spawn = require('child_process').spawn
var tmpdir = os.tmpdir()
var Pusher = require('pusher-client')
var pusher = new Pusher('bbdd9d611b463822cf6e')
var Mixpanel = require('mixpanel')
var mixpanel = Mixpanel.init('d5c191fd4468894be2824bf288879a18')
var request = require('request')
var rollbar = require('rollbar')
var open = require('open')
var TerminalManager = require('./terminal')
var tm = new TerminalManager()

// Atom shell modules.
var app = require('app')
var BrowserWindow = require('browser-window')
var Menu = require('menu')

// Set vars that control updater/client.
var versionUrl = 'http://desktop.bowery.io.s3.amazonaws.com/VERSION'
var homeVar = /^win/.test(process.platform) ? 'USERPROFILE' : 'HOME'
var ext = /^win/.test(process.platform) ? '.exe' : ''
var installDir = '..'
var binPath = path.join(__dirname, '..', 'bin')
var clientPath = path.join(binPath, 'client' + ext)
var updaterPath = path.join(binPath, 'updater' + ext)
var proc = null
var localAddr = "http://localhost:32055"
require('crash-reporter').start() // Report crashes to our server.

rollbar.init('a7c4e78074034f04b1882af596657295')

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

proc = spawn(clientPath, [])
/*
if (process.platform == 'darwin' || process.env.ENV == "no-updater") {
  proc = spawn(clientPath, [])
} else {
  proc = spawn(updaterPath, ["-d", installDir, versionUrl, "", clientPath])
}
*/
proc.on('close', function (code) {
  console.log('client process exited with code:', code)
  process.exit(code)
})
var logStream = fs.createWriteStream(path.join(process.env[homeVar], '.bowery.log'), {flags: 'a'})
proc.stdout.pipe(logStream)
proc.stderr.pipe(logStream)

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

// Attempt to get user information and register with mixpanel.
var email
var name
exec('git config user.email', function (err, stdout) {
  if (err) return

  email = stdout.replace(/(\r\n|\n|\r)/gm, '')
  exec('git config user.name', function (err, stdout) {
    if (err) return

    name = stdout.replace(/(\r\n|\n|\r)/gm, '')
    mixpanel.people.set(email, {
      $email: email,
      name: name
    })

    mixpanel.track('opened app', {
      distinct_id: email
    })

    tm.setMixpanel(mixpanel, email)
  })
})

app.on('window-all-closed', function() {
  console.log('$$$ window-all-closed')
  app.quit()
})

app.on('ready', function() {
  var template = [
    {
      label: 'Bowery', submenu: [
        {label: 'About Bowery', click: function () {open("http://bowery.io")}},
        {type: 'separator'},
      ]
    }
  ]

  var isDarwin = process.platform == 'darwin'
  var item = template[template.length - 1]
  if (isDarwin) {
    item.submenu.push.apply(item.submenu, [
      {label: 'Hide', accelerator: 'Command+H', selector: 'hide:'},
      {label: 'Quit', accelerator: 'Command+Q', click: function() {app.quit()}}
    ])
  } else {
    item.submenu.push({
      label: 'Quit', accelerator: 'Alt+F4', click: function() {app.quit()}
    })
  }


  // Set the menu items.
  template.push.apply(template, [
    {
      label: 'File',
      submenu: [
        {
          label: 'New Environment',
          accelerator: 'CommandOrControl+N',
          selector: 'new:',
          click: function() {newTerminal()}
        },
        {
          label: 'Open In Browser',
          accelerator: 'CommandOrControl+O',
          enabled: false,
          click: function () {
            var w = BrowserWindow.getFocusedWindow()
            if (w) require('open')("http://" + w.getTitle())
          }
        },
        {
          label: 'Open In File Manager',
          accelerator: 'Shift+CommandOrControl+O',
          enabled: false,
          click: function () {
            var w = BrowserWindow.getFocusedWindow()
            if (w) {
              var t = tm.getByIP(w.getTitle())
              if (t.path) require('open')(t.path)
            }
          }
        },
        {
          label: 'Info',
          accelerator: 'CommandOrControl+I',
          enabled: false,
          click: function () {
            var w = BrowserWindow.getFocusedWindow()
            if (!w) return

            var ip = w.getTitle()
            var terminal = tm.getByIP(ip)
            terminal && terminal.info()
          }
        },
        {
          label: 'Save',
          accelerator: 'CommandOrControl+S',
          enabled: false,
          click: function () {
            var w = BrowserWindow.getFocusedWindow()
            if (!w) return

            var ip = w.getTitle()
            var terminal = tm.getByIP(ip)
            terminal && terminal.save()
          }
        },
        {
          label: 'Export',
          accelerator: 'CommandOrControl+E',
          enabled: false,
          click: function () {
            var w = BrowserWindow.getFocusedWindow()
            if (!w) return

            var ip = w.getTitle()
            var terminal = tm.getByIP(ip)
            terminal && terminal.saveAndExport()
          }
        }
      ]
    },
    {
      label: 'Edit',
      submenu: [
        {
          label: 'Undo',
          accelerator: 'CommandOrControl+Z',
          selector: 'undo:',
          click: function () {
            var w = BrowserWindow.getFocusedWindow()
            if (w) {
              w.webContents.undo()
            }
          }
        },
        {
          label: 'Redo',
          accelerator: 'Shift+CommandOrControl+Z',
          selector: 'redo:',
          click: function () {
            var w = BrowserWindow.getFocusedWindow()
            if (w) {
              w.webContents.redo()
            }
          }
        },
        {
          type: 'separator'
        },
        {
          label: 'Cut',
          accelerator: 'CommandOrControl+X',
          selector: 'cut:',
          click: function () {
            var w = BrowserWindow.getFocusedWindow()
            if (w) {
              w.webContents.cut()
            }
          }
        },
        {
          label: 'Copy',
          accelerator: 'CommandOrControl+C',
          selector: 'copy:',
          click: function () {
            var w = BrowserWindow.getFocusedWindow()
            if (w) {
              w.webContents.copy()
            }
          }
        },
        {
          label: 'Paste',
          accelerator: 'CommandOrControl+V',
          selector: 'paste:',
          click: function () {
            var w = BrowserWindow.getFocusedWindow()
            if (w) {
              w.webContents.paste()
            }
          }
        },
        {
          label: 'Select All',
          accelerator: 'CommandOrControl+A',
          selector: 'selectAll:',
          click: function () {
            var w = BrowserWindow.getFocusedWindow()
            if (w) {
              w.webContents.selectAll()
            }
          }
        }
      ]
    },
    {
      label: 'View',
      submenu: [
        {
          label: 'Reload',
          accelerator: 'CommandOrControl+R',
          click: function() {
            var w = BrowserWindow.getFocusedWindow()
            w && w.reloadIgnoringCache()
          }
        },
        {
          label: 'Toggle DevTools',
          accelerator: 'Alt+CommandOrControl+I',
          click: function() {
            var w = BrowserWindow.getFocusedWindow()
            w && w.toggleDevTools()
          }
        }
      ]
    },
    {
      label: 'Window',
      submenu: [
        {
          label: 'Minimize',
          accelerator: 'CommandOrControl+M',
          selector: 'performMiniaturize:',
          click: function () {
            var w = BrowserWindow.getFocusedWindow()
            if (w) {
              w.minimize()
            }
          }
        },
        {
          type: 'separator'
        },
        {
          label: 'Bring All to Front',
          selector: 'arrangeInFront:',
          click: function () {
            var windows = BrowserWindow.getAllWindows()
            for (var i in windows) {
              windows[i].show()
            }
          }
        }
      ]
    }
  ])

  if (isDarwin) {
    template[template.length - 1].submenu.splice(1, 0, {
      label: 'Close',
      accelerator: 'Command+W',
      selector: 'performClose:',
      click: function () {
        var w = BrowserWindow.getFocusedWindow()
        if (w) {
          w.close()
        }
      }
    })
  }

  var menu = Menu.buildFromTemplate(template)
  Menu.setApplicationMenu(menu)
  tm.setMenu(menu)

  function newTerminal() {
    var paths = require('dialog').showOpenDialog({
      title: 'Where is your code?',
      properties: ['openDirectory']
    })

    var terminal
    if (paths && paths.length > 0) {
      terminal = tm.new(paths[0])
      terminal.create().fail(function (err) {
        if (err.message == "Not Connected") {
          err = "You must be connected to the internet to use Bowery. Please ensure you're connected to continue."
        } else {
          err = err.toString()
        }

        require('dialog').showErrorBox("Failed to create environment", err)
        tm.remove(terminal)
        if (tm.terminals.length <= 0) {
          app.quit()
        }
      })
    } else if (tm.terminals.length <= 0) {
      app.quit()
    }
  }

  newTerminal()
})
