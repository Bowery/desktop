// Copyright 2014 Bowery, Inc.
var fs = require('fs')
var os = require('os')
var path = require('path')
var http = require('http')
var spawn = require('child_process').spawn
var tmpdir = os.tmpdir()

exports.clientAddr = ''

// Kill any previous clients.
try {
  var pid = fs.readFileSync(path.join(tmpdir, 'bowery_client_pid'))
  if (pid) process.kill(parseInt(pid, 10), 'SIGINT')
} catch (e) {
  console.log('yolo. no pid file found.')
}

// Starts the client, cb returns the process and is triggered once a successful
// connection can be made to the client.
exports.startClient = function (cb) {
  // Start client, and run updater if it's included.
  var versionUrl = 'http://desktop.bowery.io.s3.amazonaws.com/VERSION'
  var ext = /^win/.test(process.platform) ? '.exe' : ''
  var installDir = process.platform == 'darwin' ? '../..' : '..'
  var binPath = path.join(__dirname, '..', 'bin')
  var clientPath = path.join(binPath, 'client' + ext)
  var updaterPath = path.join(binPath, 'updater' + ext)
  var opts = {stdio: 'inherit'}

  proc = spawn(updaterPath, ["-d", installDir, versionUrl, "", clientPath], opts)
  proc.on('close', function (code) {
    console.log('client process exited with code:', code)
    process.exit(code)
  })

  // Write bowery processes info.
  try {
    fs.writeFileSync(path.join(tmpdir, 'bowery_client_pid'), proc.pid)
    fs.writeFileSync(path.join(tmpdir, 'bowery_dir'), __dirname)
  } catch (e) {
    console.log(e, 'could not write pidfile')
  }

  checkClient(function () {
    return cb && cb(proc)
  })
}

// checkClient checks if the client is running and calls the callback once a
// connection is made.
function checkClient(cb) {
  var i = 0
  var retry = function () {
    if (i >= 15) {
      return cb && cb()
    }
    i++
    console.log('trying to connect to client try', i)

    http.get(exports.clientAddr + '/healthz', function (res) {
      if (res.statusCode == 200) {
        console.log('got a good response')
        return cb && cb()
      }
      console.log('failed response')

      setTimeout(function () {
        retry()
      }, 1500)
    }).on('error', function () {
      console.log('failed request')
      setTimeout(function () {
        retry()
      }, 1500)
    })
  }

  retry()
}
