# dulidu

One binary in the PATH, run from inside the repository it works on.

```sh
curl -fsSL https://raw.githubusercontent.com/thellmwhisperer/dulidu/main/install.sh | sh
dulidu init     # writes dulidu.yaml and ignores .dulidu/
dulidu doctor   # checks go, git and the config layers
dulidu update   # replaces dulidu with the latest release
```

Configuration is layered: built-in defaults, `~/.config/dulidu/config.yaml`, then the repository's `dulidu.yaml`.

Develop with `make test lint`; `make release TAG=vX.Y.Z` builds the release assets in `dist/`. macOS and Linux only.
