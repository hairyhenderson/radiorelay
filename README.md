# radio relay

The setup involves two Computers (Raspberry Pi 3), two Repeaters (Argent
Data ADS-SR1), and two Radios (QYT KT-8900D). And a bunch of wiring...

## Overview

Each side is identical:

```
   +----------+
/--| Repeater |
|  +----------+ +-------+
P     |   ^-----|   Pi  |
T     |         |       |
T     v         +-------+
|  +-------+         ^
\->| Radio |--------/
   +-------+
```

We take advantage of the UGreen USB Audio device's monitoring mode - this means
that all output from the Radio (on the Pi's USB Mic jack) will be routed back
to the Pi's USB Speaker jack, which is connected to the Repeater's Mic input.

To connect each side, we use [trx](http://www.pogo.org.uk/~mark/trx/). This
effectively multiplexes the Radio's output to the Repeater and the remote Pi.

```
+-----------+         +-----------+
|      tx ->|---UDP---|-> rx      |
| Pi 1      |         |      Pi 2 |
|      rx <-|---UDP---|<- tx      |
+-----------+         +-----------+
```

These audio channels are constantly connected, and simply act as a UDP-based
duplex relay between the two Pis.

## Software setup

The software is fairly basic, and intended to be easily reproducible, using
Docker to simplify distribution and installation.

This is all based on Raspbian Linux (Debian customized for Raspberry Pi),
with Docker CE (Community Edition) and Docker Compose installed.

### OS setup

The SD card is burned from `2017-11-29-raspbian-stretch-lite.img`, downloaded
from [here](https://downloads.raspberrypi.org/raspbian_lite_latest). I wrote
the SD card from my mac like this:

```console
$ sudo dd if=2017-11-29-raspbian-stretch-lite.img bs=1M of=/dev/rdisk2 conv=sync
$
```

To run from Linux the device name (for `of=`) will need to change.

Before removing the SD card, there's a few things to do:

#### Pre-boot setup

##### Enable SSH

```console
$ touch /boot/ssh
$
```

##### Set up WiFi (optional)

To auto-connect to a WiFi network, create a file `wpa_supplicant.conf` in `/boot`
that looks like:

```
ctrl_interface=/var/run/wpa_supplicant
ap_scan=1
network={
	ssid="<ssid name>"
	psk=<key (can be a hash)>
}
```

It's best to hash the PSK instead of storing it plain-text. This can be generated 
with the `wpa_passphrase` command:

```console
$ wpa_passphrase myssid foobarbaz
network={
	ssid="myssid"
	#psk="foobarbaz"
	psk=91a242f3a64a7004260c1947eb3293a9ea2e8c70e71476e44af779227fb830ab
}
```

Just smush that together with the listing above.

#### Post-boot setup

Eject the SD card and pop it in the Pi, then boot it up. You'll be able to SSH to
`raspberrypi.local` after a suitable period (up to ~1min). The username is `pi`
and default password is `raspberry`.

```console
$ ssh pi@raspberrypi.local
...
```

_Also handy is to copy over your SSH public key with `ssh-copy-id`, but that's_
_an exercise left to the reader._

##### Set the hostname

`raspberrypi` isn't the best hostname, especially when there are multiple on the network:

```console
$ export HOSTNAME=myhostname
$ echo $HOSTNAME | sudo tee /etc/hostname
myhostname
$ sudo sed -i "s/raspberrypi/$HOSTNAME/" /etc/hosts
```

##### Package installs

A bunch of packages/keys are necessary/useful to bootstrap things:

```console
$ cat <<EOF > /etc/apt/sources.list.d/docker.list
deb [arch=armhf] https://download.docker.com/linux/raspbian/ stretch stable
EOF
$ curl -fsSL https://download.docker.com/linux/$(. /etc/os-release; echo "$ID")/gpg | sudo apt-key add -
...
$ sudo apt-get update && sudo apt-get install avahi-daemon docker-ce git
...
```

##### Installing docker-compose

```console
$ docker create --name compose hairyhenderson/docker-compose:1.17.1-armhf
...
$ sudo docker cp compose:/docker-compose /usr/local/bin/docker-compose
$ docker rm compose
$ docker-compose -v
docker-compose version 1.17.1, build a9597d7
```

### Configuration

#### Audio levels

To set the audio levels, use `alsamixer` to get the values right. Trial-and-error
is really the only option here, though a speaker level of ~86 seems reasonable.
Bear in mind that `alsamixer` starts in Playback view by default, and the view
should be changed to "All" view with the `F5` key (or you can start with
`alsamixer -V=all`).

Notes:
- Mic should be unmuted
  - the bottom of the Mic column should read `OO`, not `MM`
- Mic Capture (the second Mic column) should be enabled
  - hit `<Space>` until `CAPTURE` is displayed at the bottom of the second Mic column
- exit and save with `<Esc>`

Once audio levels are OK, save them:

```console
$ sudo alsactl store
$
```

This will store the level configuration in `/var/lib/alsa/asound.state`, and
should be restored on boot.

#### trx

`tx` and `rx` are run in Docker containers, using Docker Compose. They're
managed with the `docker-compose` command, and configured with a file named
`docker-compose.yml`, which lives in `/home/pi`.

To start the services you can do (from `pi`'s home directory):

```console
$ docker-compose up -d
```

To tail (follow) the command outputs:

```console
$ docker-compose logs -f
```

To stop services:

```console
$ docker-compose down
```

##### The `docker-compose.yml` file

```yaml
version: '2.3'

x-common: &common
  image: hairyhenderson/trx:arm
  restart: always
  privileged: true
  ulimits:
    rtprio: 25
  devices:
    - /dev/snd:/dev/snd
  network_mode: host

services:
  tx:
    <<: *common
    command: tx -d plughw:CARD=Device,DEV=0 -h 10.0.1.22 -c 1 -r 48000 -f 480
    labels:
      description: Input from the local radio, transmit to the remote repeater
  rx:
    <<: *common
    command: rx -d plughw:CARD=Device,DEV=0 -c 1 -r 48000 -j 60
    labels:
      description: Receive from the remote radio, output to the local repeater
```

Write this file (modifying the IP address in the `tx` command) to `/home/pi/docker-compose.yml`.
Whitespace is significant!

##### Starting on boot

After the Pi boots, it'll run `docker-compose up -d` automatically, so no interaction
should be necessary. To accomplish this, we configure a SystemD unit (as root!):

```console
$ cat <<EOF | sudo tee /etc/systemd/system/trx.service >/dev/null
[Unit]
Description=trx services
After=network.target docker.socket
Requires=docker.socket

[Service]
Type=oneshot
RemainAfterExit=yes
WorkingDirectory=/home/pi
ExecStart=/usr/local/bin/docker-compose up -d
ExecStop=/usr/local/bin/docker-compose down
TimeoutStartSec=0

[Install]
WantedBy=multi-user.target
EOF
$ sudo systemctl enable trx.service
$ sudo systemctl daemon-reload
```
