var s3 = require('s3')
var AWS = require('aws-sdk')
var awsS3Client = new AWS.S3()
awsS3Client.config.update({accessKeyId: 'AKIAIH32JWYHAMXUBHGA', secretAccessKey: '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'})
var params = {
  s3Params: {
    Bucket: 'boweryapps'
  }
}

var client = s3.createClient({s3Client: awsS3Client})

module.exports = function (prefix, dir, cb) {
  cb = cb || function () {}
  params.localDir = dir
  params.s3Params.Prefix = prefix
  var uploader = client.uploadDir(params)
  uploader.on('error', cb)
  uploader.on('progess', function () {
    console.log('progess', uploader.progressAmount)
  })
  uploader.on('end', cb)
}
