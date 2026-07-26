# Contributing Guide

Run platform-neutral unit tests:

```bash
make test
```

End-to-end tests require a device connected to the network.

```bash
export C64U_ADDRESS="192.168.1.100"  # Defaults to hostname "c64u" if unset
export C64U_PASSWORD="your_password" # Optional: Password if set on the device

make e2e
```

By default, `make e2e` runs in verbose mode (`-v`) to show active test execution. You can filter which tests to run using `TEST_RUN=<pattern>`:

```bash
make e2e TEST_RUN=TestE2E_KeyboardPress
```

Some test cases require manual verification through keyboard or joystick input.
Some tests require an Ethernet connection rather than WiFi.

Run `golangci-lint` across the codebase:

```bash
make lint
```
