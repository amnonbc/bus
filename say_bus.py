import datetime
import subprocess
import next_bus
import argparse

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


def expected_to_string(usecs):
    minutes = int(next_bus.minutes_till_bus(usecs))
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
                                          expected_to_string(bus['EstimatedTime'])))
        for bus in buses[1:args.num_busses]:
            say('And the one after that is %s.' % expected_to_string(bus['EstimatedTime']))

if __name__ == "__main__":
    parser = argparse.ArgumentParser()
    parser.add_argument('-r', "--route", help="bus route", type=int, default=102)
    parser.add_argument('-s', "--stop", help="bus stop id", default=74640)
    parser.add_argument('-n', "--num_busses", help="number of busses to report", type=int, default=3)
    parser.add_argument('-v', "--voice", help="voice", default='en-f1')
    args = parser.parse_args()

    buses = next_bus.get_bus_times(args.route, args.stop)
    say_times(buses)
