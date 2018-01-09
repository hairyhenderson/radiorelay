# radio relay

Listing what the audio devices are on the pi:

```console
$ aplay -l
...
```

These correspond to `plughw:N,0` (where N is the index)

on the card where the radio is plugged in:

## Legend

- A: local radio's audio out (to sound card 1's red/mic jack)
- B: local repeater's audio out (to sound card 2's red/mic jack)

- C: remote radio's audio out (to sound card 1's red/mic jack)
- D: remote repeater's audio out (to sound card 2's red/mic jack)

## Audio routing

From the radio's audio out, the sound must go _only_ to the repeaters (local and remote)
From the repeater's audio out, the sound must go _only_ to the radios (local and remote)

To hook up local repeater to local radio, we use `alsaloop`:

(`plughw:2,0` is the repeater audio card, `plughw:0,0` is the radio audio card)

```console
$ alsaloop -C plughw:2,0 -P plughw:0,0
$ alsaloop -C plughw:0,0 -P plughw:2,0
$
```

To hook up local repeater to remote radio, we use `tx`:

```console
$ tx -d plughw:2,0 -h <remote IP address> -c 1 -r 48000 -f 480
...
```

TO hook up remote repeater to local radio, we use `rx`:

```console
$ rx -d plughw:0,0 -c 1 -r 48000 -j 60
...
```

## PTT routing

From the repeater's PTT out, state must be propagated to local and remote radios.

2 pins: 18 (radio PTT in), 27 (repeater PTT out)

- when 27 is detected active:
  - set 18 active
  - transmit PTT activate message to remote IP
- when 27 is detected inactive:
  - set 18 inactive
  - transmit PTT deactivate message to remote IP

## Misc. stuff

### OS setup

The SD card is burned from `2017-11-29-raspbian-stretch-lite.img`, downloaded
from [here](https://downloads.raspberrypi.org/raspbian_lite_latest). I wrote
the SD card on my mac like this:

```console
$ sudo dd if=2017-11-29-raspbian-stretch-lite.img bs=1M of=/dev/rdisk2 conv=sync
$
```

Before removing the SD card, there's a few things to do:

#### Pre-boot setup

##### Enable SSH

```console
$ touch /boot/ssh
$
```

##### Make some kernel commandline additions

```console
$ sudo sed -i 's/$/ cgroup_memory=1 modules-load=snd-aloop/' /boot/cmdline.txt
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

_Also handy is to run `ssh-copy-id`, but that's out of scope._

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
$ sudo apt-get update && sudo apt-get install avahi-daemon docker-ce git
...
$ curl -fsSL https://download.docker.com/linux/$(. /etc/os-release; echo "$ID")/gpg | sudo apt-key add -
...
```

##### Installing docker-compose

This helps run things!

```console
$ docker create --name compose hairyhenderson/docker-compose:1.17.1-armhf
...
$ sudo docker cp compose:/docker-compose /usr/local/bin/docker-compose
$ docker rm compose
$ docker-compose -v
docker-compose version 1.17.1, build a9597d7
```