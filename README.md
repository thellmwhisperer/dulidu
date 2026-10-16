# dulidu

One binary in the PATH, run from inside the repository it works on.

```sh
curl -fsSL https://raw.githubusercontent.com/thellmwhisperer/dulidu/main/install.sh | sh
dulidu init     # writes dulidu.yaml and ignores .dulidu/
dulidu doctor   # checks go, git and the config layers
dulidu update   # replaces dulidu with the latest release
dulidu release vX.Y.Z # builds and publishes a tagged release
```

Configuration is layered: built-in defaults, `~/.config/dulidu/config.yaml`, then the repository's `dulidu.yaml`.

Develop with `make test lint`; `make dist TAG=vX.Y.Z` builds assets without publishing, and `make release TAG=vX.Y.Z` publishes them. macOS and Linux only.
