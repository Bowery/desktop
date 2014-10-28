// Source for the executable at desktop.bowery.io/rmmanifest.zip
#include <iostream>

#include <windows.h>

int main(int argc, char *argv[]) {
  if (argc < 2) {
    std::cerr << "Exe file required as an argument" << std::endl;
    return 1;
  }

  HANDLE handle = BeginUpdateResourceA(argv[1], FALSE);
  if (handle == NULL) {
    std::cerr << GetLastError() << std::endl;
    return 1;
  }

  BOOL res = UpdateResourceA(handle, RT_MANIFEST, MAKEINTRESOURCEA(1), 1033, NULL, 0);
  if (!res) {
    std::cerr << GetLastError() << std::endl;
    return 1;
  }

  if (!EndUpdateResourceA(handle, false)) {
    std::cerr << GetLastError() << std::endl;
    return 1;
  }

  return 0;
}
