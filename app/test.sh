#!/bin/bash
# wails-dev-wrapper.sh

# The location of your libusb.dylib
LIBUSB_PATH=$(pwd)/../libusb.dylib

# Copy libusb.dylib to the current directory
cp "$LIBUSB_PATH" ./libusb.dylib

# This is the tricky part - we need to hook into the process that creates wailsbindings
# Create a monitor script that will copy the dylib to the temp directory
cat >/tmp/wails-monitor.sh <<'EOF'
#!/bin/bash
while true; do
  # Look for newly created wailsbindings files
  BINDINGS_DIR=$(find /private/var/folders -name wailsbindings -type f 2>/dev/null | head -1)
  if [ ! -z "$BINDINGS_DIR" ]; then
    TEMP_DIR=$(dirname "$BINDINGS_DIR")
    # Copy the libusb.dylib to the same directory
    cp ./libusb.dylib "$TEMP_DIR/libusb.dylib"
    echo "Copied libusb.dylib to $TEMP_DIR"
    sleep 5
  fi
  sleep 1
done
EOF

chmod +x /tmp/wails-monitor.sh

# Start the monitor script in the background
/tmp/wails-monitor.sh &
MONITOR_PID=$!

# Run wails dev
wails dev

# Clean up
kill $MONITOR_PID
