# dulidu

One binary in the PATH. Run `init`, `doctor`, `runs`, and `release` inside a Git repository.

```sh
curl -fsSL https://raw.githubusercontent.com/thellmwhisperer/dulidu/main/install.sh | sh
dulidu init     # writes dulidu.yaml and ignores .dulidu/
dulidu doctor   # checks go, git and the config layers
dulidu doctor --agent # asks the configured agent to say OK and reports elapsed time
dulidu runs     # lists .dulidu/runs/ newest first: label, status, last step, started
dulidu update   # updates an installed release to the latest release
dulidu release vX.Y.Z # builds and publishes a tagged release
```

Configuration is layered: built-in defaults, `~/.config/dulidu/config.yaml`, then the repository's `dulidu.yaml`.
Set the agent CLI, model, effort and timeout in the `dulidu.yaml` template written by `dulidu init`. The agent uses its installed CLI and existing authentication.

`update` leaves development builds untouched. Before building, `release` checks that an existing tag on `origin` points to the current commit.

Develop with `make test lint`; `make dist TAG=vX.Y.Z` builds assets without publishing, and `make release TAG=vX.Y.Z` publishes them. macOS and Linux only.
