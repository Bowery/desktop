#!/usr/bin/env bash

install="/usr/local/bowery"
shortcut="${HOME}/Desktop/Bowery.desktop"

sudo mkdir -p "${install}"
sudo cp -rf * "${install}"
sudo chmod +x "${install}/bowery"
sudo rm -f "/usr/local/bin/bowery"
sudo ln -sf "${install}/bowery" "/usr/local/bin/bowery"

for i in $(find "${install}/resources/"{bin,app}); do
  if [[ -d "${i}" ]] || [[ -x "${i}" ]]; then
    sudo chmod 0777 "${i}"
  else
    sudo chmod 0666 "${i}"
  fi
done

echo "[Desktop Entry]" > "${shortcut}"
echo "Encoding=UTF-8" >> "${shortcut}"
echo "Version=1.0" >> "${shortcut}"
echo "Name=Bowery" >> "${shortcut}"
echo "GenericName=Bowery" >> "${shortcut}"
echo "Exec=/usr/local/bowery/bowery" >> "${shortcut}"
echo "Terminal=false" >> "${shortcut}"
echo "Icon=/usr/local/bowery/logo.png" >> "${shortcut}"
echo "Type=Application" >> "${shortcut}"
chmod +x "${shortcut}"
cp -rf "${shortcut}" "${HOME}/.local/share/applications/Bowery.desktop"
