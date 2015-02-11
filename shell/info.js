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
        '<div class="primary">' + c.name + '</div>',
        '<div class="secondary">' + c.email + '</div>',
      '</li>'
    ].join('\n'))
  })
  peopleElContent.push('</ul>')
  peopleEl.innerHTML = peopleElContent.join('\n')
})
