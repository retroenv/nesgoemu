## Installation of GUI dependencies

The following libraries need to be installed, depending on the operating system:

### **macOS**

Xcode or Command Line Tools for Xcode:

```
xcode-select --install
```

### **Ubuntu/Debian-like**

For SDL support:

```
apt install libsdl2{,-image,-mixer,-ttf,-gfx}-dev
```

### **CentOS/Fedora-like**

For SDL support:

```
yum install SDL2{,_image,_mixer,_ttf,_gfx}-devel
```

### Windows

For SDL support:

1. Install [msys2](http://www.msys2.org/)
2. Start msys2 and execute:
```
pacman -S --needed base-devel mingw-w64-i686-toolchain mingw-w64-x86_64-toolchain mingw64/mingw-w64-x86_64-SDL2
```
3. Add `c:\tools\msys64\mingw64\bin\` to the user path environment variable
