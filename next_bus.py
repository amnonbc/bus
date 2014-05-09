#!/usr/bin/python
# A library for accessing TFL countdown times

import json
import datetime
import argparse
import sys
import collections

import geopy
import geopy.geocoders
import geopy.distance
import requests


def ms_timestamp_to_date(ts):
    return datetime.datetime.fromtimestamp(int(ts)/1000)


def minutes_till_bus(ts):
    t = ms_timestamp_to_date(ts)
    n = datetime.datetime.now()
    delta = t-n
    return delta.total_seconds()/60

# response types from TFL API
STOP_ARRAY = 0
BUS_PREDICTION = 1

# TFL say that they will return the data in this order _irrespective_ of the oder specified
# in the ReturnList argument
RETURNABLE_FIELDS = ['StopPointName', 'StopID', 'StopCode1', 'StopCode2', 'StopPointState',
                  'StopPointType', 'StopPointIndicator', 'Towards', 'Bearing', 'Latitude',
                  'Longitude', 'VisitNumber', 'TripID', 'VehicleID', 'RegistrationNumber',
                  'LineID', 'LineName', 'DirectionID', 'DestinationText', 'DestinationName',
                  'EstimatedTime', 'MessageUUID', 'MessageText', 'MessageType', 'MessagePriority',
                  'StartTime', 'ExpireTime', 'BaseVersion']


def _sort_to_tfl_order(fields):
    fields.sort(key=lambda f: RETURNABLE_FIELDS.index(f))


def _parse_bus_response(requested_fields, response_type, lines):
    """
    Countdown responses consist of a sequence of JSON arrays
    Each array consists of a tag, followed by a list of values corresponding to the
    requested fields.
    :param requested_fields: fields requested
    :param response_type: type of data to parse
    :param lines: line iterator over http response body
    :return: list of dict
    """
    res = []
    for line in lines:
        try:
            j = json.loads(line)
            if j.pop(0) == response_type:
                res.append(dict(zip(requested_fields, j)))
        except:
            pass  # TFL returned malformed JSON :-(
    return res


def _get_countdown_data(selectors, response_type, requested_fields):
    BUS_BASE_URL = "http://countdown.api.tfl.gov.uk/interfaces/ura/instant_V1"
    _sort_to_tfl_order(requested_fields)
    selectors['ReturnList'] = ','.join(requested_fields)
    p = requests.get(BUS_BASE_URL, params=selectors)
    if p.status_code != requests.codes.ok:
        p.raise_for_status()
    return _parse_bus_response(requested_fields, response_type, p.iter_lines())


def get_bus_times(stop_code, bus_num=None, with_destination=False):
    """
    return arrival times for bus_num at stop_id
    :param stop_code: of bus stop of interest
    :param bus_num: number of bus, of none if we want data for all buses
    :return: array of dicts of bus attributes, sorted in order of arrival time
    """
    requested_fields = ['LineName', 'EstimatedTime']
    if with_destination:
        requested_fields.append('DestinationText')
    selectors = {'StopCode1': stop_code}
    if bus_num:
        selectors['LineName'] = bus_num
    response = _get_countdown_data(selectors, BUS_PREDICTION, requested_fields)
    for b in response:
        b['when'] = ms_timestamp_to_date(b['EstimatedTime'])
    return sorted(response, key=lambda b: b['when'])


def get_bus_stops(bus_line):
    """
    Returns an list of bus stops for bus_line
    :param bus_line:
    :return: list of dicts - one dict for each bus stop
    """
    requested_fields = ['StopPointName', 'StopCode1', 'Towards']
    selectors = {'LineName': bus_line}
    return _get_countdown_data(selectors, STOP_ARRAY, requested_fields)


def get_routes_for_stops(stops):
    requested_fields = ['StopCode1', 'LineName']
    selectors = {'StopCode1': ','.join(stops)}
    res = _get_countdown_data(selectors, BUS_PREDICTION, requested_fields)
    m = collections.defaultdict(set)
    for r in res:
        m[r['StopCode1']].add(r['LineName'])
    return m


def get_bus_stops_near(location):
    """
    Returns an list of bus stops near specified location
    :param location: post code
    :return: list of dicts - one dict for each bus stop
    """
    geo = geopy.geocoders.GoogleV3()
    _, loc = geo.geocode(location)
    requested_fields = ['StopPointName', 'StopCode1', 'Towards', 'Latitude', 'Longitude']
    selectors = {'Circle': '%g,%g,500' % loc}
    stops = _get_countdown_data(selectors, STOP_ARRAY, requested_fields)
    routes = get_routes_for_stops([s['StopCode1'] for s in stops])
    for s in stops:
        if s['StopCode1'] in routes:
            s['routes'] = routes[s['StopCode1']]
        s['dist'] = geopy.distance.distance(loc, (s['Latitude'], s['Longitude']))

    return sorted(stops, key=lambda s: s['dist'])


def _write_buses(buses):
    for b in buses:
        print "%3s %20s %6s" % (b['LineName'], b['DestinationText'], b['when'].strftime('%H:%M:%S'))


def _write_stops(stops):
    for s in stops:
        towards = s['Towards']
        if 'dist' in s:
            print '%dm,' % s['dist'].meters,
        print '%s, %s' % (s['StopCode1'], s['StopPointName']),
        if 'routes' in s and s['routes']:
            print '(%s),' % ', '.join(sorted(s['routes'], key=int)),
        if towards:
            print 'towards ', towards,
        print


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description='Print predicted arrival times for TFL buses')
    parser.add_argument('-r', "--route", help="bus route", type=int, default=None)
    parser.add_argument('-p', "--postcode", help="find stops within 500m of post code", default=None)
    parser.add_argument('-s', "--stop", help="bus stop id", default=74640)
    parser.add_argument('-l', "--list_stops", help="list all bus stops for route", action='store_true')
    args = parser.parse_args()

    if args.postcode:
        _write_stops(get_bus_stops_near(args.postcode))
    elif args.list_stops:
        if not args.route:
            sys.exit('--route argument must be specified')
        _write_stops(get_bus_stops(args.route))
    else:
        buses = get_bus_times(args.stop, args.route, with_destination=True)
        _write_buses(buses)