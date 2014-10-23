# Bowery Desktop
Set up your development environment in 30 seconds flat.

## Important Commands
- `make` will boot up the app just as the user would boot it up.
- `make ui` will recompile just the front end so you can make small changes without restarting the app (as long as the web inspector is open and caching is turned off).
- `make agent` will release the bowery agent into the wild. It will publish itself as whatever version is in `bowery/agent/VERSION`.
- `make client` will release the desktop app into the wild. It will publish itself as whatever version is in `bowery/client/VERSION`.
- `make release` will release the agent and the client into the wild.
- `make clean` will remove all compiled artifacts except Bowery.app (because it takes a long time to download)
- `make extra-clean` removes everything. If you do this, the next time you run `make` will take a long time.

## Directory Structure
- `/bin` is where things get compiled to
- `/bowery` has the go apps for the `client`, `agent`, and `updater`.
- `/build` is where the Bowery.app we run in development is actually located
- `/scripts` are a set of utilities that you should never call directly, but are used by the Makefile
- `/shell` is where the [atom-shell](github.com/atom/atom-shell) app that interacts with atom shell is. When we run the app in development we pass its location as a parameter to Bowery.app
- `/ui` the front end [polymer](polymer-project.org) app.
- `circle.yml` tells cirlceci how to run tests / deploy the app
- `Makefile` has a bunch of commands for running, testing, releasing, and cleaning the bowery desktop app. Don't run commands if you're not sure what they do. It could result in breaking the live bowery.
- `debug.log` is where the makefile writes its output to. You can also see application logs there. I highly recommend `tail -f debug.log` while you're developing.
