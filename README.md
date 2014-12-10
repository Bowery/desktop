# Bowery Desktop
Set up your development environment in 30 seconds flat.

## Important Commands
- `make` will boot up the app just as the user would boot it up on OS X.
- `make agent` will release the bowery agent into the wild. It will publish itself as whatever version is in `bowery/agent/VERSION`.
- `make client` will release the desktop app into the wild. It will publish itself as whatever version is in `bowery/client/VERSION`.
- `make clean` will remove all compiled artifacts except Bowery.app (because it takes a long time to download)
- `make extra-clean` removes everything. If you do this, the next time you run `make` will take a long time.

## Directory Structure
- `/bin` is where things get compiled to
- `/client` runs on the users computer, watches files for changes, and syncs them to `agent`.
- `/updater` is an app used to update the `client`. It is started by `shell` and starts `client`.
- `/build` is where the Bowery.app we run in development is actually located
- `/scripts` are a set of utilities that you should never call directly, but are used by the Makefile
- `/shell` is where the [atom-shell](github.com/atom/atom-shell) app lives.
- `circle.yml` tells cirlceci how to run tests / deploy the app
- `Makefile` has a bunch of commands for running, testing, releasing, and cleaning the bowery desktop app. Don't run commands if you're not sure what they do. It could result in breaking the live bowery.
- `debug.log` is where the makefile writes its output to. You can also see application logs there. I highly recommend `tail -f debug.log` while you're developing.
