#!/usr/bin/python

import datetime
import subprocess
import argparse

import next_bus






# As the raspberry pi comes with no display, I thought that speaking the bus times through the audio port
# would be a convenient.


DEVNULL = open('/dev/null', 'w')


def say(txt):
    print txt
    try:
        subprocess.call(['espeak', '-ven+f5', '-k5', txt], stderr=DEVNULL)
    except:
        pass


def int2bus(route):
    if route <= 100:
        return route
    res = list(str(route).replace('0', 'O'))
    return ' '.join(res)


def expected_to_string(tm):
    minutes = int((tm - datetime.datetime.now()).total_seconds()/60)
    if minutes == 1:
        return 'in 1 minute'
    elif minutes:
        return 'in %d minutes' % minutes
    else:
        return 'due now'


def say_times(buses):
    say(datetime.datetime.now().strftime('%H:%M'))
    if buses:
        bus = buses[0]
        say('The next %s bus to %s is %s.' % (int2bus(bus['LineName']), bus['DestinationText'],
                                          expected_to_string(bus['when'])))
        for bus in buses[1:args.num_buses]:
            say('And the one after that is %s.' % expected_to_string(bus['when']))

if __name__ == "__main__":
    parser = argparse.ArgumentParser(description='say expected arrival times of next buses')
    parser.add_argument('-r', "--route", help="bus route", type=int, default=102)
    parser.add_argument('-s', "--stop", help="bus stop id", default=74640)
    parser.add_argument('-n', "--num_buses", help="number of buses to report", type=int, default=2)
    parser.add_argument('-v', "--voice", help="voice", default='en-f1')
    args = parser.parse_args()

    buses = next_bus.get_bus_times(args.stop, args.route, with_destination=True)
    say_times(buses)
