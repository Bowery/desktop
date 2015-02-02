// Copyright 2014 Bowery, Inc.
/**
 * @fileoverview Progress manager for loading screens.
 */


/**
 * @constructor
 * @param {string} id The identifier for this container.
 * @param {string}
 */
function ProgressManager (id, type) {
  if (ProgressManager.Types.indexOf(type) == -1)
    throw new Error('invalid type')

  this._id = id
  this._type = type
}

/**
 * @enum {string}
 */
ProgressManager.TypeLaunching = 'launching'

/**
 * @enum {string}
 */
ProgressManager.TypeSaving = 'saving'

/**
 * @enum {string}
 */
ProgressManager.TypeExporting = 'exporting'

/**
 * @enum {Array<string>}
 */
ProgressManager.Types = [
  ProgressManager.TypeLaunching,
  ProgressManager.TypeSaving,
  ProgressManager.TypeExporting
]

/**
 * @enum {Array<string>}
 */
ProgressManager.StepsLaunching = [
  'environment'
]

/**
 * @enum {Array<string>}
 */
ProgressManager.StepsSaving = [
  'environment'
]

/**
 * @enum {Array<string>}
 */
ProgressManager.StepsExporting = [
  'environment'
]

/**
 * @param {string} The identifier for this container.
 */
ProgressManager.prototype._id = ''

/**
 * @param {string} The type of progress being monitored.
 *    This can be creating, saving, exporting.
 */
ProgressManager.prototype._type = ''

/**
 * @param {Array} steps
 */
ProgressManager.prototype.steps = []

/**
 * @param {Object} bar element.
 */
ProgressManager.prototype.barEl = null

/**
 * @param {Object} description element.
 */
ProgressManager.prototype.descriptionEl = null

/**
 * @param {Object} error element.
 */
ProgressManager.prototype.errorEl = null

/**
 * @param {Object} Pusher channel.
 */
ProgressManager.prototype.subChan = null

/**
 * init sets the elements and subscribes to
 * the pusher channel.
 */
ProgressManager.prototype.init = function () {
  this.barEl = document.querySelector('.progress-bar')
  this.descriptionEl = document.querySelector('.progress-description')
  this.errorEl = document.querySelector('.progress-error')
  this.setSteps()
  this.subscribe()
}

/**
 * setSteps sets the steps for this particular manager.
 */
ProgressManager.prototype.setSteps = function () {
  switch (this._type) {
    case ProgressManager.TypeLaunching:
      this.steps = ProgressManager.StepsLaunching
      break
    case ProgressManager.TypeSaving:
      this.steps = ProgressManager.StepsSaving
      break
    case ProgressManager.TypeExporting:
      this.steps = ProgressManager.StepsExporting
      break
  }
}

/**
 * subscribe subscribes the this container's event
 * channel and binds to progress and error events.
 */
ProgressManager.prototype.subscribe = function () {
  this.subChan = pusher.subscribe('container-' + this._id)
  this.subChan.bind('progress', this.handleProgressEvent.bind(this))
  this.subChan.bind('error', this.handleErrorEvent.bind(this))
}

/**
 * handleProgressEvent handles new progress events.
 * @param {Object} event data.
 */
ProgressManager.prototype.handleProgressEvent = function (data) {
  var contents = []
  try {
    contents = data.split(':')
  } catch (e) {}

  this.updateProgress(contents[0], parseFloat(contents[1]))
}

/**
 * updateProgress updates the progress display.
 */
ProgressManager.prototype.updateProgress = function (step, prog) {
  var index = this.steps.indexOf(step)
  if (!~index) return

  var existingProg = (index / this.steps.length) * 100
  var addedProg = (prog / this.steps.length) * 100

  this.descriptionEl.innerHTML = this._type + ' ' + step
  this.barEl.style.width = existingProg + addedProg + '%'
}

/**
 * handleErrorEvent handles new error events.
 * @param {Object} event data.
 */
ProgressManager.prototype.handleErrorEvent = function (data) {
  this.showError(data.error)
}

/**
 * showError displays a new error.
 */
ProgressManager.prototype.showError = function (err) {
  this.errorEl.innerHTML = err
  this.errorEl.className = 'progress-error'
  this.descriptionEl.className = 'progress-description hidden'
}

/**
 * hideError hides the error element.
 */
ProgressManager.prototype.hideError = function () {
  this.errorEl.className = 'progress-error hidden'
  this.descriptionEl.className = 'progress-description'
}
