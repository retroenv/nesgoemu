# GUI Dependencies

nesgoemu can run in two modes:
- Console mode (`-c` flag): No additional dependencies required
- GUI mode (default): Requires SDL2 libraries

The GUI uses SDL2 for cross-platform window management, input handling, and rendering. You'll need to install SDL2 development libraries for your platform.

## Platform-Specific Instructions

### macOS

Install SDL2 using Homebrew:

```bash
brew install sdl2
```

### Ubuntu/Debian

Install SDL2 development packages:

```bash
sudo apt install libsdl2{,-image,-mixer,-ttf,-gfx}-dev
```

### CentOS/Fedora/RHEL

Install SDL2 development packages:

```bash
sudo yum install SDL2{,_image,_mixer,_ttf,_gfx}-devel
```

For newer versions, use `dnf` instead of `yum`.

### Windows

1. Install [MSYS2](https://www.msys2.org/)
2. Open MSYS2 terminal and install SDL2:
   ```bash
   pacman -S --needed base-devel mingw-w64-x86_64-toolchain mingw64/mingw-w64-x86_64-SDL2
   ```
3. Add `C:\msys64\mingw64\bin` to your system PATH environment variable

### Other Platforms

For other Unix-like systems, install SDL2 development libraries using your package manager. The package names are typically `libsdl2-dev`, `SDL2-devel`, or similar.

## Troubleshooting

If you encounter runtime errors when using GUI mode:

1. Verify SDL2 libraries are installed and in your system's library path
2. On Linux, you may need additional packages like `libsdl2-2.0-0`
3. On macOS, ensure SDL2 is installed via Homebrew or your package manager

## Console Mode

nesgoemu can run without GUI using the `-c` flag. SDL2 libraries are only required when actually running in GUI mode, not for building the binary.
