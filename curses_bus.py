#!/usr/bin/python

import curses
import argparse
import next_bus
import time
import datetime
import os

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
    pass


def write_console(stdscr, buses, nlines):
    for i, b in enumerate(buses[0:nlines]):
        mins_left = next_bus.expected_short(b['EstimatedTime'])
        stdscr.addstr(i, 0, "%3s %6s" % (b['LineName'], mins_left))
    write_time(stdscr)
    stdscr.refresh()


def write_time(stdscr):
    stdscr.addstr(3, 2, datetime.datetime.now().strftime('%H:%M:%S'))
    curses.curs_set(0)


def main_loop(args):
    stdscr = init_console()
    if os.getuid() == 0:
        os.setuid(1000)
    while True:
        try:
            buses = next_bus.get_bus_times(args.route, args.stop)
            stdscr.clear()
        except:
            pass
        for i in range(args.delay):
            write_console(stdscr, buses, args.num_busses)
            time.sleep(1)


if __name__ == "__main__":
    parser = argparse.ArgumentParser()
    parser.add_argument('-r', "--route", help="bus route", type=int, default=102)
    parser.add_argument('-s', "--stop", help="bus stop id", default=74640)
    parser.add_argument('-n', "--num_busses", help="number of busses to report", type=int, default=3)
    parser.add_argument('-d', "--delay", help="seconds to wait in between updates", type=int, default=30)
    args = parser.parse_args()
    main_loop(args)
