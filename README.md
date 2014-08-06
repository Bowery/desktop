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
The Makefile automates this process `make` to start the app and `make clean` to stop it. You can see logs in `debug.log`.
