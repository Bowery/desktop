# Project Mercer
Dropbox for Dev Environments

## Important Commands
- `make` will boot up the app just as the user would boot it up and a dev version of the server
- `make release` will release the app into the wild.
- `make clean` will remove all compiled artifacts except Bowery.app (because it takes a long time to download)

## Directory Structure
- `/bin` is where things get compiled to
- `/bowery` has the go apps for the `client`, `server`, and `updater`.
- `/build` is where the Bowery.app we run in development is actually located
- `/scripts` are a set of utilities that you should never call directly, but are used by the Makefile
- `/shell` is where the [atom-shell](github.com/atom/atom-shell) app that interacts with atom shell is. When we run the app in development we pass its location as a parameter to Bowery.app
- `/ui` the front end [polymer](polymer-project.org) app.
- `circle.yml` tells cirlceci how to run tests / deploy the app
- `Makefile` has a bunch of commands for running, testing, releasing, and cleaning the bowery desktop app. Don't run commands if you're not sure what they do. It could result in breaking the live bowery.
- `debug.log` is where the makefile writes its output to. You can also see application logs there. I highly recommend `tail -f debug.log` while you're developing.

## Routes
