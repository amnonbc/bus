#!/usr/bin/python

import curses
import argparse
import time
import datetime
import os

import next_bus







# This script provides a running textual display of the next bus arrivals.
# It is inspired by the electronic notice boards above some London bus stops.
# But it uses a 10x5 character screen which has a much squarer shape than the TFL signs.

def init_console():
    # redirect stdin and stdout to console
    cons = os.open('/dev/tty1', os.O_RDWR)
    os.dup2(cons, 0)
    os.dup2(cons, 1)
    # tell curses the size of the terminal
    os.environ['COLS'] = '10'
    os.environ['LINES'] = '5'
    s = curses.initscr()
    curses.curs_set(0)
    unblank_screen()
    s.clear()
    return s


def uninit_console():
    curses.endwin()


def unblank_screen():
    print "\033[9;0]"


def expected_short(delta):
    minutes = int((delta).total_seconds()/60)
    if not minutes:
        return 'Due'
    if minutes < 0:
        return 'Gone'
    return str(minutes) + ' min'


def write_console(stdscr, buses, nlines, status):
    now = datetime.datetime.now()
    for i, b in enumerate(buses[0:nlines]):
        mins_left = expected_short(b['when'] - now)
        stdscr.addstr(i, 0, "%3s %6s" % (b['LineName'], mins_left))
    write_time(stdscr)
    write_status(stdscr, status)
    stdscr.refresh()


def write_time(stdscr):
    stdscr.addstr(3, 2, datetime.datetime.now().strftime('%H:%M:%S'))
    curses.curs_set(0)

def write_status(stdscr, stat):
    stdscr.addstr(3, 0, stat[0])


def main_loop(args):
    stdscr = init_console()
    num_consqutive_failures = 0
    # Maybe better to use gEvent etc. than invent my own timing loop
    buses = []
    while True:
        try:
            if args.test:
                raise Exception('testing')
            write_status(stdscr, 'U')
            stdscr.refresh()
            buses = next_bus.get_bus_times(args.stop, args.route)
            status = ' '
            num_consqutive_failures = 0
            stdscr.clear()
        except:
            # mini watchdog - attempt to recover when network down
            num_consqutive_failures += 1
            status = str(num_consqutive_failures)
            if num_consqutive_failures > 10:
                os.system('reboot')
            elif num_consqutive_failures > 4:
                os.system('/usr/sbin/service networking restart')

        for i in range(args.delay):
            # should perhaps use the ExpiresTime provided by TFL, rather than a user-settable
            # polling delay
            now = datetime.datetime.now()
            # delete buses which have gone
            while buses[0]['when'] > now:
                buses.pop(0)
                stdscr.clear()
            write_console(stdscr, buses, args.num_busses, status)
            time.sleep(1)


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description='Display a board of next arrival times for TFL buses')
    parser.add_argument('-r', "--route", help="bus route", type=int, default=102)
    parser.add_argument('-s', "--stop", help="bus stop id", default=74640)
    parser.add_argument('-n', "--num_busses", help="number of busses to report", type=int, default=3)
    parser.add_argument('-d', "--delay", help="seconds to wait in between updates", type=int, default=30)
    parser.add_argument('-t', "--test", help="test failure recover", action="store_true")
    args = parser.parse_args()
    main_loop(args)
