// Copyright 2014 Bowery, Inc.
var fs = require('fs')
var os = require('os')
var path = require('path')
var http = require('http')
var spawn = require('child_process').spawn
var tmpdir = os.tmpdir()
var Pusher = require('pusher-client')
var stathat = require('stathat')
var pusher = new Pusher('bbdd9d611b463822cf6e')
var request = require('request')

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
var lastFilePath

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

if (process.platform == 'darwin' || process.env.ENV == "no-updater") {
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
  console.log('$$$ window-all-closed')
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
      label: 'File',
      submenu: [
        {
          label: 'Open In File Manager',
          accelerator: 'CommandOrControl+O',
          click: function () {
            if (lastFilePath) require('open')(lastFilePath)
          }
        },
        {
          label: 'Open In Browser',
          accelerator: 'CommandOrControl+O',
          click: function () {
            var w = BrowserWindow.getFocusedWindow()
            if (w) require('open')("http://" + w.getTitle())
          }
        },
        {
          label: 'New Environment',
          accelerator: 'CommandOrControl+N',
          selector: 'new:',
          click: function() {newContainer()}
        },
        {
          label: 'Export',
          accelerator: 'CommandOrControl+E',
          click: function () {
            var w = BrowserWindow.getFocusedWindow()
            if (!w) return

            var ip = w.getTitle()
            request("http://localhost:32055/env/" + ip, function(err, res, body) {
              if (err) return

              var bson = JSON.parse(body)
              if (bson.status != 'success') return

              // aiyo david
            })
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
        }
      ]
    }
  ]
  var menu = Menu.buildFromTemplate(template)
  Menu.setApplicationMenu(menu)

  // newContainer prompts the user for an application to launch
  // an environment with and listens for an update from Pusher.
  // On successful launch, initiate SSH connection to the container.
  function newContainer () {
    var paths = require('dialog').showOpenDialog({
      title: 'Where is your code?',
      properties: ['openDirectory']
    })
    if (paths && paths.length > 0) {
      lastFilePath = paths[0]
      var req = http.request({
        host: 'localhost',
        port: 32055,
        method: 'POST',
        path: '/containers',
        headers: {
          'Content-Type': 'application/json'
        }
      }, function (response) {
        // On successful response, create window.
        mainWindow = new BrowserWindow({
          title: 'Bowery',
          frame: true,
          width: 570,
          height: 370,
          show: true,
          resizable: true
        })

        var body = ''
        response.on('data', function (chunk) {
          body += chunk
        })

        response.on('end', function () {
          var data = JSON.parse(body.toString())
          var container = data.container

          mainWindow.loadUrl('file://' + path.join(__dirname, 'loading.html?container_id=' + container._id))
          var channel = pusher.subscribe('container-' + container._id)

          channel.bind('error', function (data) {
            mainWindow.send('error', data)
          })

          channel.bind('update', function (data) {
            setTimeout(function () {
              openSSH(data._id, data.address, data.user, data.password)
            }, 500)
          })
        })
      })

      req.write(JSON.stringify({localPath: paths[0]}))
      req.end()
    } else {
      // Pressed "Cancel" so just exit.
      app.quit()
    }
  }

  // openSSH initiates an ssh connection to the container and
  // sets window bindings for close related events.
  function openSSH (id, ip, user, password) {
    var start = Date.now()
    var query = require('url').format({
      query: {
        ip: ip, user: user, password: password
      }
    })

    mainWindow.loadUrl('file://' + path.join(__dirname, 'term.min.html?' + query))
    mainWindow.setTitle(ip)

    // hterm changes window title when cwd changes triggering
    // a page-title-updated event. Override default behavior
    // and ignore change.
    mainWindow.on('page-title-updated', function (e) {
      e.preventDefault()
    })

    // When the window is closed, prompt the user to "save"
    // the environment.
    mainWindow.on('close', function (e) {
      // Returns a value in [0, 1, 2] corresponding to a button.
      e.preventDefault()
      var confirm = require('dialog').showMessageBox(mainWindow, {
        type: 'warning',
        buttons: ['Save', 'Don\'t save', 'Cancel'],
        message: 'Do you want to save the changes you made to this environment?',
        detail: 'Your changes will be lost if you don\'t save them.'
      })

      console.log('$$$ confirm', confirm)

      // If the user selects save, run save and then delete requests.
      // Destroy and remove reference to window afterwards. If the user
      // selects not to save, run delete request and window remove async.
      switch (confirm) {
      case 0:
        saveContainer(id, function () {
          deleteContainer(id, function () {
            endSession(start)
            mainWindow.destroy()
            mainWindow = null
          })
        })
        break
      case 1:
        deleteContainer(id, function () {
          endSession(start)
          mainWindow.destroy()
          mainWindow = null
        })
        break
      }
    })
  }

  // todo(steve): show save progress.
  function saveContainer (id, cb) {
    var req = http.request({
      host: 'localhost',
      port: 32055,
      method: 'PUT',
      path: '/containers/' + id
    }, function (response) {
      response.on('data', function (chunk) {
        console.log('$$$ data', chunk)
      })

      response.on('end', function () {
        cb && cb()
      })
    })

    req.write('')
    req.end()
  }

  function deleteContainer (id, cb) {
    var req = http.request({
      host: 'localhost',
      port: 32055,
      method: 'DELETE',
      path: '/containers/' + id
    }, function (response) {
      response.on('data', function (chunk) {
        console.log('$$$ data', chunk)
      })

      response.on('end', function () {
        cb && cb()
      })
    })

    req.write('')
    req.end()
  }

  // endSession posts the elapsed ssh time to StatHat.
  function endSession (startTime) {
    var end = Date.now()
    stathat.trackEZValue('tibJDdtL7nf5dRIB', 'desktop ssh elapsed time', end - startTime,
      function (status, json) {
        console.log(status, json)
      }
    )
  }

  newContainer()
})
