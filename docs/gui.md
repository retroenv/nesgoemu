# GUI Dependencies

nesgoemu can run in two modes:

- Console mode (`-c`): no additional runtime libraries required.
- GUI mode (default): requires SDL2 runtime libraries.

The Go build itself does not require CGO. GUI mode uses SDL2 through retrogolib for window setup,
input handling, and rendering.

## Platform-Specific Setup

### macOS

Install SDL2 with Homebrew:

```bash
brew install sdl2
```

### Ubuntu/Debian

Install SDL2 packages:

```bash
sudo apt install libsdl2{,-image,-mixer,-ttf,-gfx}-dev
```

### Fedora/RHEL

Install SDL2 packages:

```bash
sudo dnf install SDL2{,_image,_mixer,_ttf,_gfx}-devel
```

On older RHEL-derived systems, use `yum` instead of `dnf`.

### Windows

1. Install [MSYS2](https://www.msys2.org/).
2. Open an MSYS2 terminal.
3. Install SDL2:

```bash
pacman -S --needed base-devel mingw-w64-x86_64-toolchain mingw64/mingw-w64-x86_64-SDL2
```

Add `C:\msys64\mingw64\bin` to your system `PATH` so the SDL2 DLLs can be found at runtime.

### Other Platforms

Install SDL2 using your platform package manager. Package names are commonly `libsdl2-dev`,
`SDL2-devel`, or `sdl2`.

## Console Mode

Use console mode when SDL2 is not installed, when running in CI, or when capturing traces:

```bash
nesgoemu -c game.nes
nesgoemu -c -t game.nes > trace.log
```

## Troubleshooting

If GUI mode fails at startup:

1. Verify SDL2 is installed.
2. Verify SDL2 shared libraries are in the runtime library path.
3. On Linux, install the runtime package as well as development headers when your distribution splits them.
4. On Windows, verify the MSYS2 `mingw64\bin` directory is on `PATH`.

For non-GUI workflows, use `-c` to bypass SDL2 setup entirely.
