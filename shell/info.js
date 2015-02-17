function qmark (name) {
  var queryString = {}
  window.location.href.replace(
    new RegExp("([^?=&]+)(=([^&]*))?", "g"),
    function($0, $1, $2, $3) { queryString[$1] = $3 }
  )
  return name ? queryString[name] : queryString
}

var generalEl = document.getElementById('general')
var peopleEl = document.getElementById('people')
var generalBtn = document.getElementById('general-btn')
var peopleBtn = document.getElementById('people-btn')

generalBtn.onclick = function (e) {
  generalBtn.className = 'active'
  peopleBtn.className = ''
  generalEl.className = ''
  peopleEl.className = 'hidden'
}

peopleBtn.onclick = function (e) {
  generalBtn.className = ''
  peopleBtn.className = 'active'
  generalEl.className = 'hidden'
  peopleEl.className = ''
}

var baseURL = 'http://localhost:32055'
var projectID = qmark('project_id')
var request = require('request')
request({
  url: baseURL + '/projects/' + projectID,
  method: 'GET'
}, function (err, res, body) {
  if (err)
    return

  var data = JSON.parse(body)

  generalEl.innerHTML = [
    '<ul>',
      '<li>',
        '<div class="primary">Address</div>',
        '<div class="secondary">' + qmark('address') + '</div>',
      '</li>',
      '<li>',
        '<div class="primary">SSH Port</div>',
        '<div class="secondary">' + qmark('ssh_port') + '</div>',
      '</li>',
      '<li>',
        '<div class="primary">Username</div>',
        '<div class="secondary">' + qmark('username') + '</div>',
      '</li>',
      '<li>',
        '<div class="primary">Password</div>',
        '<div class="secondary">' + qmark('password') + '</div>',
      '</li>',
    '</ul>',
  ].join('\n')

  peopleElContent = []
  peopleElContent.push('<ul>')
  data.project.collaborators.forEach(function (c) {
    peopleElContent.push([
      '<li>',
        '<input type="hidden" value=' + c._id + '/>',
        '<div class="primary">' + c.name + '</div>',
        '<div class="secondary">' + c.email + '</div>',
        '<div class="permissions">',
          '<div class="permission">',
            '<label>Can Save Environment</label>',
            '<input type="checkbox" name="canEdit" ' + ((c.permissions && c.permissions.canEdit) ? 'checked' : '') + '>',
          '</div>',
        '</div>',
      '</li>'
    ].join('\n'))
  })
  peopleElContent.push('</ul>')
  peopleElContent.push('<div class="update-error"></div>')
  peopleElContent.push('<a href="#" id="update-btn" class="btn skeleton">Update</a>')
  peopleEl.innerHTML = peopleElContent.join('\n')

  var updateBtn = document.getElementById('update-btn')
  var updateErr = document.getElementsByClassName('update-error')[0]
  updateBtn.onclick = function (e) {
    e.preventDefault()

    updateBtn.innerHTML = 'Updating...'

    var collaborators = document.querySelectorAll('#people li')
    for (var i = 0; i < collaborators.length; i++) {
      var id = collaborators[i].children[0].value
      var permissions = collaborators[i].querySelector('.permissions')
      for (var j = 0; j < permissions.children.length; j++) {
        var p = permissions.children[j]
        if (!data.project.collaborators[i].permissions)
          data.project.collaborators[i].permissions = {}

        data.project.collaborators[i].permissions[p.children[1].name] = p.children[1].checked
      }
    }

    data.project.id = projectID
    request({
      url: baseURL + '/projects/' + projectID,
      method: 'PUT',
      body: JSON.stringify(data.project)
    }, function (err, res, body) {
      if (body.error)
        updateErr.innerHTML = body.error

      setTimeout(function () {
        updateBtn.innerHTML = 'Update'
      }, 250)
    })
  }
})
