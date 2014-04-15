#!/usr/bin/python

import json
import datetime
import subprocess
import argparse

import requests


def ms_timestamp_to_date(ts):
    return datetime.datetime.fromtimestamp(int(ts)/1000)


def minutes_till_bus(ts):
    t = ms_timestamp_to_date(ts)
    n = datetime.datetime.now()
    delta = t-n
    return delta.total_seconds()/60

def parse_bus_response(requested_fields, lines):
    """
    Countdown requests unfortunately do not return JSON.
    Instead they return a sequence of JSON arrays
    Each array consists of a tag, followed by a list of values corresponding to the
    requested fields. (Why could they not have just returned a JSON object?)
    :param requested_fields: fields requested
    :param response_text: http response body
    :return: list of dict sorted by bus arrival time
    """
    res = []
    for line in lines:
        j = json.loads(line)
        if j.pop(0) == 1:
            res.append({field: val for field, val in zip(requested_fields, j)})
    return sorted(res, key=lambda b: b['EstimatedTime'])


def get_bus_times(bus_num, stop_id):
    requested_fields = ['LineName', 'DestinationText', 'EstimatedTime']
    BUS_BASE_URL = "http://countdown.api.tfl.gov.uk/interfaces/ura/instant_V1"
    filter = {
        'StopCode1': stop_id,
        'LineName': bus_num,
        'ReturnList': ','.join(requested_fields)}
    p = requests.get(BUS_BASE_URL, params=filter)
    if p.status_code != requests.codes.ok:
        p.raise_for_status()
    return parse_bus_response(requested_fields, p.iter_lines())


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
    minutes = int(minutes_till_bus(usecs))
    if minutes == 1:
        return 'in 1 minute'
    elif minutes:
        return 'in %d minutes' % minutes
    else:
        return 'due now'

def expected_short(usec):
    minutes = int(minutes_till_bus(usec))
    if not minutes:
        return 'Due'
    if minutes < 0:
        return 'Gone'
    return str(minutes) + ' min'



def write_console(buses):
    print "\033[9;0]" # unblank console
    subprocess.call(['clear'])
    for b in buses[0:2]:
        print "%3s %6s" % (b['LineName'], expected_short(b['EstimatedTime']))


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
    parser.add_argument('-c', "--console", help="run in console mode", action='store_true')
    parser.add_argument('-r', "--route", help="bus route", type=int, default=102)
    parser.add_argument('-s', "--stop", help="bus stop id", default=74640)
    parser.add_argument('-n', "--num_busses", help="number of busses to report", type=int, default=3)
    parser.add_argument('-v', "--voice", help="voice", default='en-f1')
    args = parser.parse_args()

    buses = get_bus_times(args.route, args.stop)

    if args.console:
        write_console(buses)
    else:
        say_times(buses)

