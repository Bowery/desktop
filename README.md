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

## Database
Uses orchestrate and schemas can be found in `bowery/server/schemas`. All keys will be orchestrate generated ids which are also stored as fields for good measure.

## Developer
Our version of a user model.
- `name:string`
- `email:string`
- `password:string`
- `salt:string`

## MetaFile
- localPath:string
- s3Path:string
- version:string
- md5:string

## Env
An array of `MetaFile`'s that need to be synced. Individual files are not stored in their own collection with their own keys.
- `files:[]MetaFile`

## Team
Information about what `Developer`'s are sharing what `Env`.
- `path:[]string` (we will merge everyone's $PATH)
- `members:[]Developer.id`
- `master:Env.id`
- `creator:Developer`

## Routes
### Server
- `POST /signup` creates a `Developer`, a new `Team` with the local `Env` from scratch.
- `PATCH /team/:teamId` adds someone to the `Team`.
- `GET /pull/:teamId` responds with the `Team`'s master `Env`.
- `PATCH /push/:teamId` updates the `Team`'s master `Env`.

### Client
- `POST /auth` logs in the user and responds with their `Team`'s Id. If the user was not part of a team according to the `Server`, it will create a team.
- `GET /subscribe/:teamId` tells the client what to show over SSE.
- `GET /pull` will work with the `Server` to make the local `Env` match the `Team`'s master `Env`.
- `GET /push` will tell the `Server` to make the local `Env` the `Team`'s master `Env`. They may have to pull first.

## UI Screens
- `/auth` prompts the user for login/signup credentials. if successful, it will respond with the `Team`'s Id and will request `GET /subscribe/:teamId` which will decide which screens are shown.
- `/pull` will show the screen asking the user to pull down and apply the `Team`'s master `Env`.
- `/push` will show the screen asking the user to push up their changes to update the `Team`'s master `Env`.
- `/default` just says that everything is up to date.
