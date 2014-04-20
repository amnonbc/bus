#!/usr/bin/python

import json
import datetime
import argparse
import pprint

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

def parse_bus_response(requested_fields, response_type, lines):
    """
    Countdown requests unfortunately do not return JSON.
    Instead they return a sequence of JSON arrays
    Each array consists of a tag, followed by a list of values corresponding to the
    requested fields. (Why could they not have just returned a JSON object?)
    :param requested_fields: fields requested
    :param response_type: type of data to parse
    :param response_text: http response body
    :return: list of dict sorted by bus arrival time
    """
    res = []
    for line in lines:
        j = json.loads(line)
        if j.pop(0) == response_type:
            res.append({field: val for field, val in zip(requested_fields, j)})
    return res


def get_coundown_data(filter, response_type, requested_fields):
    BUS_BASE_URL = "http://countdown.api.tfl.gov.uk/interfaces/ura/instant_V1"
    p = requests.get(BUS_BASE_URL, params=filter)
    if p.status_code != requests.codes.ok:
        p.raise_for_status()
    return parse_bus_response(requested_fields, response_type, p.iter_lines())


def get_bus_times(stop_code, bus_num=None):
    """
    return arrival times for bus_num at stop_id
    :param stop_code: of bus stop of interest
    :param bus_num: number of bus, of none if we want data for all busses
    :return: array of dicts of bus attributes, sorted in order of arrival time
    """
    requested_fields = ['LineName', 'DestinationText', 'EstimatedTime']
    filter = {
        'StopCode1': stop_code,
        'ReturnList': ','.join(requested_fields)}
    if bus_num:
        filter['LineName'] = bus_num
    response = get_coundown_data(filter, BUS_PREDICTION, requested_fields)
    return sorted(response, key=lambda b: b['EstimatedTime'])


def get_bus_stops(bus_num):
    requested_fields = ['StopPointName', 'StopCode1', 'Towards']
    filter = {
        'LineName': bus_num,
        'ReturnList': ','.join(requested_fields)}
    return get_coundown_data(filter, STOP_ARRAY, requested_fields)

def write_busses(buses):
    for b in buses:
        print "%3s %20s %6s" % (b['LineName'], b['DestinationText'],
                                ms_timestamp_to_date(b['EstimatedTime']).strftime('%H:%M:%S'))


if __name__ == "__main__":
    parser = argparse.ArgumentParser()
    parser.add_argument('-r', "--route", help="bus route", type=int, default=None)
    parser.add_argument('-s', "--stop", help="bus stop id", default=74640)
    parser.add_argument('-l', "--list_stops", help="list all bus stops for route", action='store_true')
    args = parser.parse_args()

    if args.list_stops:
        pprint.pprint(get_bus_stops(args.route))
    else:
        buses = get_bus_times(args.stop, args.route)
        write_busses(buses)

