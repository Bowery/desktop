// Copyright 2015 Bowery, Inc.
/**
 * @fileoverview TerminalManager and Terminal objects. Operates the
 * lifecycle of a terminal session, including creating, saving,
 * and deleting.
 */

var request = require('request')
var path = require('path')
var Q = require('kew')
var baseURL = 'http://localhost:32055'
var Pusher = require('pusher-client')
var pusherC = new Pusher('bbdd9d611b463822cf6e')
var BrowserWindow = require('browser-window')

/**
 * TerminalManager maintains the state for all
 * active Terminal sessions.
 * @constructor
 */
function TerminalManager() {}

/**
 * @enum {Array<Terminal>} active terminals.
 */
TerminalManager.prototype.terminals = []

/**
 * @enum {Object} menu.
 * @private
 */
TerminalManager.prototype._menu = null

/**
 * new creates a new terminal and adds it to the list
 * of active terminals.
 * @param {string} path to code
 * @return {Terminal}
 */
TerminalManager.prototype.new = function (path) {
  var t = new Terminal(path)
  t.setDelegate(this)
  return this.add(t)
}

/**
 * add adds a terminal.
 * @param {Terminal} terminal object.
 */
TerminalManager.prototype.add = function (terminal) {
  this.terminals.push(terminal)
  return terminal
}

/**
 * getByIP returns the terminal with matching ip address.
 * @return {Terminal} 
 */
TerminalManager.prototype.getByIP = function (ip) {
  for (var i = 0; i < this.terminals.length; i++)
    if (this.terminals[i].container.address == ip)
      return this.terminals[i]
}

/**
 * remove removes a terminal.
 * @param {Terminal} terminal object.
 */
TerminalManager.prototype.remove = function (terminal) {
  for (var i = 0; i < this.terminals.length; i++)
    if (this.terminals[i].container._id == terminal.container._id)
      this.terminals.splice(i, 1)
}

/**
 * setMenu sets the menu.
 * @param {Object} menu
 */
TerminalManager.prototype.setMenu = function (menu) {
  this._menu = menu
}

/**
 * getMenu gets the menu.
 * @return {Object} menu
 */
TerminalManager.prototype.getMenu = function () {
  return this._menu
}

/**
 * updateSubmenuItem updates a sub menu item.
 * @param {string} label Top level label.
 * @param {string} sub Label within top level label.
 * @param {string} key
 * @param {string|bool} value
 */
TerminalManager.prototype.updateSubmenuItem = function (label, sub, key, value) {
  var menu = this.getMenu()
  for (var i = 0; i < menu.items.length; i++) {
    if (menu.items[i].label == label) {
      for (var j = 0; j < menu.items[i].submenu.items.length; j++) {
        if (menu.items[i].submenu.items[j].label == sub) {
          menu.items[i].submenu.items[j][key] = value
        }
      }
    }
  }
}

/**
 * Terminal represents a session. It operates the window
 * and has methods for all lifecycle events.
 * @param {string} path Path to files
 * @constructor
 */
function Terminal (path) {
  this.path = path
}

/**
 * The local path of the application.
 * @enum {string}
 */
Terminal.prototype.path = ''

/**
 * Container object.
 * @enum {Object}
 */
Terminal.prototype.container = {}

/**
 * Pusher channel.
 * @enum {Object}
 * @private
 */
Terminal.prototype._subChan = null

/**
 * hterm window.
 * @enum {Object}
 * @private
 */
Terminal.prototype._window = null

/**
 * Delegate
 * @enum {TerminalManager}
 * @private
 */
Terminal.prototype._delegate = null

/**
 * setDelegate sets the delegate.
 * @param {TerminalManager}
 */
Terminal.prototype.setDelegate = function (delegate) {
  this._delegate = delegate
}

/**
 * getDelegate returns the delegate.
 * @return {TerminalManager}
 */
Terminal.prototype.getDelegate = function () {
  return this._delegate
}

/**
 * Send an http request.
 * @param {string} path
 * @param {string} method
 * @param {Object} body
 * @return {Promise}
 * @private
 */
Terminal.prototype._req = function (path, method, body) {
  var defer = Q.defer()
  var opts = {
    url: baseURL + path,
    method: method
  }
  if (body) opts.body = JSON.stringify(body)

  request(opts, defer.makeNodeResolver())
  return defer.promise
}

/**
 * create creates a new container.
 * @return {Promise}
 */
Terminal.prototype.create = function () {
  console.log('[DEBUG]: creating container')
  return this._req('/containers', 'POST', {
    localPath: this.path
  })
  .then(this._handleCreateRes.bind(this))
  .fail(this._handleCreateErr.bind(this))
}

/**
 * save saves a container.
 * @return {Promise}
 */
Terminal.prototype.save = function () {
  console.log('[DEBUG]: saving container')
  if (!this.container._id)
    throw new Error('an active container is required')

  return this._req('/containers/' + this.container._id, 'PUT')
  .then(this._handleSaveRes.bind(this))
  .fail(this._handleSaveErr.bind(this))
}

/**
 * delete deletes a container.
 * @return {Promise}
 */
Terminal.prototype.delete = function () {
  console.log('[DEBUG]: deleting container')
  var id = this.container._id
  if (!id)
    throw new Error('an active container is required')

  return this._req('/containers/' + id, 'DELETE')
  .then(this._handleDeleteRes.bind(this))
  .fail(this._handleDeleteErr.bind(this))
}

/**
 * saveAndDelete saves the container and then deletes it.
 * It redirects to the progress page during this operation.
 */
Terminal.prototype.saveAndDelete = function() {
  var query = require('url').format({
    query: {
      type: 'saving',
      container_id: this.container._id
    }
  })

  this._window.loadUrl('file://' + path.join(__dirname, 'progress.html?' + query))
  var self = this
  request({
    url: baseURL + '/containers/' + this.container._id,
    method: 'PUT'
  }, function (err, res, body) {
    self._subChan.on('saved', function (data) {
      request({
        url: baseURL + '/containers/' + self.container._id,
        method: 'DELETE'
      }, function (err, res, body) {
        self.getDelegate().remove(self)
        self._window.destroy()
      }) 
    })
  })
}

/**
 * export shows the user export steps for the container.
 */
Terminal.prototype.export = function () {
  var self = this
  request({
    url: baseURL + '/env/' + this.container.address,
    method: 'GET'
  }, function (err, res, body) {
    if (err)
      return

    var data = JSON.parse(body)
    if (data.status != 'success')
      return

    var confirm = require('dialog').showMessageBox(self._window, {
      type: 'info',
      buttons: ['Docker', 'Shell', 'Cancel'],
      message: 'Select a format to export to.',
      detail: 'You can pipe this container into docker load or download and mount it directly without Docker.'
    })

    if (confirm == 2)
      return

    var content = confirm == 0 ? data.docker : data.shell
    require('clipboard').writeText(content)

    require('dialog').showMessageBox(self._window, {
      type: 'info',
      buttons: ['OK'],
      message: 'Copied to clipboard!',
      detail: content
    })
  })
}

/**
 * _handleCreateRes handles the create http response
 * and if successful sets the container, creates a new
 * window, and subscribes to updates via Pusher. Returns
 * the container.
 * @param {Object} res
 * @return {Object}
 * @private
 */
Terminal.prototype._handleCreateRes = function (res) {
  var body = JSON.parse(res.body.toString())
  if (body.error)
    throw new Error(body.error)

  this.container = body.container
  this._createWindow()
  this._subscribe()

  // show loading screen
  var query = require('url').format({
    query: {
      type: 'launching',
      container_id: this.container._id
    }
  })

  this._window.loadUrl('file://' + path.join(__dirname, 'progress.html?' + query))
  return this
}

/**
 * _handleSaveRes
 * @param {Object} res
 * @private
 */
Terminal.prototype._handleSaveRes = function (res) {
  var body = JSON.parse(res.body.toString())
  if (body.error)
    throw new Error(body.error)
}

/**
 * _handleDeleteRes
 * @param {Object} res
 * @private
 */
Terminal.prototype._handleDeleteRes = function (res) {
  var body = JSON.parse(res.body.toString())
  if (body.error)
    throw new Error(body.error)

  this.getDelegate().remove(this)
  this._window.destroy()
  return this
}

/**
 * _handleCreateErr
 * @param {Error}
 * @private
 */
Terminal.prototype._handleCreateErr = function (err) {
  return err
}

/**
 * _handleSaveErr
 * @param {Error}
 * @private
 */
Terminal.prototype._handleSaveErr = function (err) {
  return err
}

/**
 * _handleDeleteErr
 * @param {Error}
 * @private
 */
Terminal.prototype._handleDeleteErr = function (err) {
  return err
}

/**
 * _subscribe subscribes to `created` and `update` events
 * for the container.
 * @private
 */
Terminal.prototype._subscribe = function () {
  var id = this.container._id
  if (!id)
    throw new Error('an active container is required')

  console.log('[DEBUG]: subscribing to pusher for updates')
  this._subChan = pusherC.subscribe('container-' + id)
  this._subChan.bind('created', this._handleCreateEvent.bind(this))
  this._subChan.bind('update', this._handleUpdateEvent.bind(this))
}

/**
 * _handleCreateEvent handles create event data from
 * Pusher. On successful create the window will redirect
 * to the terminal page and establish an ssh connection.
 * @param {Object} event data
 * @private
 */
Terminal.prototype._handleCreateEvent = function (data) {
  console.log('[DEBUG]: received `create` event from pusher')

  this.container = data
  this.connect()
  this.getDelegate().updateSubmenuItem('File', 'Export', 'enabled', true)
  this.getDelegate().updateSubmenuItem('File', 'Open In Browser', 'enabled', true)
  this.getDelegate().updateSubmenuItem('File', 'Open In File Manager', 'enabled', true)
}

/**
 * _handleUpdteEvent
 * @param {Object} event data
 * @private
 */
Terminal.prototype._handleUpdateEvent = function (data) {
  console.log('[DEBUG]: received `update` event from pusher')
}

/**
 * _createWindow creates a new window.
 * @private
 */
Terminal.prototype._createWindow = function () {
  console.log('[DEBUG]: creating window')
  this._window = new BrowserWindow({
    title: 'Bowery',
    frame: true,
    width: 570,
    height: 370,
    show: true,
    resizable: true
  })
}

/**
 * _handleWindowClose handles an attempt to close the terminal window.
 * It prompts the user with a dialog, asking whether to save, don't save,
 * or cancel. Executes appropriate lifecycle event based on user input.
 * @param {Object} event
 */
Terminal.prototype._handleWindowClose = function (e) {
  e.preventDefault()
  var confirm = require('dialog').showMessageBox(this._window, {
    type: 'warning',
    buttons: ['Save', 'Don\'t save', 'Cancel'],
    message: 'Do you want to save the changes you made to this environment?',
    detail: 'Your changes will be lost if you don\'t save them.'
  })

  switch (confirm) {
    // Save & Delete.
    case 0:
      this.saveAndDelete()
      break
    // Delete.
    case 1:
      this.delete()
      break
    // Cancel.
    case 2:
      break
  }
}

/**
 * connect creates the SSH connection and redirects the window to the
 * terminal view. 
 */
Terminal.prototype.connect = function () {
  var ip = this.container.address
  var user = this.container.user
  var password = this.container.password
  var query = require('url').format({
    query: {
      ip: ip,
      user: user,
      password: password
    }
  })

  this._window.loadUrl('file://' + path.join(__dirname, 'term.min.html?' + query))
  this._window.setTitle(ip)
  this._window.on('page-title-updated', function (e) {
    e.preventDefault()
  })
  this._window.on('close', this._handleWindowClose.bind(this))
}

module.exports = TerminalManager
