# bus

<img width="434" alt="Screenshot 2023-12-13 at 06 45 12" src="https://github.com/amnonbc/bus/assets/7283149/7755bdeb-f894-4c08-a951-1003bbcb1666">

This a app I run on a Pi2 which displays the next busses arriving at the nearest bus stop [TFL countdown](http://countdown.tfl.gov.uk/) API.

I wrote this in Python a decade ago. But it stopped working because [TFL countdown](http://countdown.tfl.gov.uk/) was no longer 
prepared to communitate using now deprecated TLS ciphers. Python 2.7 is no longer supported on the Raspberry Pi, and upgrading
to Python 3 was a pain so I just rewrote the thing in Go. The old Python 
code has been moved to the [pibus](https://github.com/amnonbc/bus/tree/master/pybus) directory.

