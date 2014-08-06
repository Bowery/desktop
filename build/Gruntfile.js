module.exports = function(grunt) {

  // Project configuration.
  grunt.initConfig({
    pkg: grunt.file.readJSON('package.json'),
    "download-atom-shell": {
      version: "0.15.3",
      outputDir: "./atom-shell",
      rebuild: true
    },
    'build-atom-shell-app': {
      options: {
        atom_shell_version: 'v0.15.3',
        platforms: ['darwin', 'win32', 'linux']
      }
    }
  })

  grunt.loadNpmTasks('grunt-download-atom-shell')
  grunt.loadNpmTasks('grunt-atom-shell-app-builder')

}
