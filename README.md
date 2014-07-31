# Bowery Desktop Client
It's important that you use Bowery to develop Bowery as much as possible so that you'll develop empathy for users and catch bugs early on.


## Agent
The artist formally known as the artist formally known as Satellite should be developed on a remote server using a stable build of the OS X Toolbar app.

1. Create a box with Golang installed on Digital Ocean or AWS
2. Create a new Application in the toolbar app with the following start/build commands.
```
Start: ./dev-agent -env=development
Build: ./build.sh
```

## Client
Once you read how to setup the MenubarApp in XCode you'll realize that its a terrible development experience. Instead you should run the client in your browser.

In bowery/client:
```
$ ENV=development gin # Uses Stable Agent
$ ENV=development AGENT=development gin # Uses Agent you're developing
```
Then navigate to `localhost:3000`.

## MenubarApp
The `BoweryMenubarApp` directory can be opened as a project in XCode.

1. Stop any running version of the application
2. Run clean in XCode  - black magic that solves most problems
3. `cd bowery && make` - builds the go client app and move it to xcode project
4. `pkill client`      - XCode won't kill it on its own)
5. Run the XCode Project

Rinse and Repeat...

The Makefile automates this process `make` to start the app and `make clean` to stop it. You can see logs in `debug.log`.
