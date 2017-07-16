#!/usr/bin/python

import argparse
import datetime
import signal
import sys
import time

import pygame
from pygame.locals import *

import next_bus

WHITE = (250, 250, 250)

RED = (10, 0, 10)


def expected_short(delta):
    ts = delta.total_seconds()
    minutes, seconds = divmod(ts, 60)
    if ts < 15:
        return 'Due'
    if minutes < 0:
        return 'Gone'
    return '%2d:%02d' % (minutes, seconds)


def write_console(background, buses, nlines, status, shift=0):
    background.fill((255, 255, 255,  255))

    now = datetime.datetime.now()
    for i, b in enumerate(buses[0:nlines]):
        mins_left = expected_short(b['when'] - now)
        s = "%3s %6s" % (b['LineName'], mins_left)
        upd(background, s, i, shift)
    write_status(background, status)
    write_time(background)
    screen.blit(background, (0, 0))
    pygame.display.flip()


def write_time(background):
    s = datetime.datetime.now().strftime('%H:%M:%S')
    text = STATUS_FONT.render(s, 1, (10, 10, 10))
    textpos = text.get_rect()
    textpos.right = background.get_rect().w
    textpos.bottom = background.get_rect().h
    background.blit(text, textpos)


def write_status(background, s):

    text = STATUS_FONT.render(s, 1, (255, 0, 0))
    textpos = text.get_rect()
    textpos.bottom = background.get_rect().h
    textpos.left = 20
    background.blit(text, textpos)


def upd(background, s, n, shift):
    text = BIG_FONT.render(s, 1, RED)
    textpos = text.get_rect()
    h = textpos.h
    textpos.top = n * h  - shift
    textpos.left = 20
    background.blit(text, textpos)


def main_loop(args, background, screen):
    num_consqutive_failures = 0
    # Maybe better to use gEvent etc. than invent my own timing loop
    buses = []
    while True:
        for event in pygame.event.get():
            if event.type == QUIT:
                return

        try:
            if args.test:
                raise Exception('testing')
            write_status(background, 'U')
            screen.blit(background, (0, 0))
            pygame.display.flip()
            now = datetime.datetime.now()
            if now.hour < 9:
                route = args.route
            else:
                route = 0
            buses = next_bus.get_bus_times(args.stop, route)
            status = ' '
            num_consqutive_failures = 0
        except:
            # mini watchdog - attempt to recover when network down
            num_consqutive_failures += 1
            status = str(num_consqutive_failures)

        for i in range(args.delay):
            # should perhaps use the ExpiresTime provided by TFL, rather than a user-settable
            # polling delay
            now = datetime.datetime.now()
            # delete buses which have gone
            if buses and buses[0]['when'] < now:
                for shift in range(159):
                    write_console(background, buses, args.num_busses + 1, status, shift)
                buses.pop(0)
            write_console(background, buses, args.num_busses, status)
            time.sleep(1)



def main():
    parser = argparse.ArgumentParser(description='Display a board of next arrival times for TFL buses')
    parser.add_argument('-r', "--route", help="bus route", type=int, default=102)
    parser.add_argument('-s', "--stop", help="bus stop id", default=74640)
    parser.add_argument('-n', "--num_busses", help="number of busses to report", type=int, default=3)
    parser.add_argument('-d', "--delay", help="seconds to wait in between updates", type=int, default=30)
    parser.add_argument('-t', "--test", help="test failure recover", action="store_true")
    args = parser.parse_args()

    # Fill background
    background = pygame.Surface(screen.get_size())
    background = background.convert()
    background.fill(WHITE)

    # Event loop
    main_loop(args, background, screen)


def signal_handler(signal, frame):
  time.sleep(1)
  pygame.quit()
  sys.exit(0)


if __name__ == '__main__':
    pygame.init()
    pygame.display.init()
    pygame.font.init()
    signal.signal(signal.SIGTERM, signal_handler)
    signal.signal(signal.SIGINT, signal_handler)
    info = pygame.display.Info()
    size = (info.current_w, info.current_h)
    STATUS_FONT = pygame.font.Font(None, 90)
    BIG_FONT = pygame.font.Font(None, info.current_h/3 + 20)

    screen = pygame.display.set_mode(size)
    pygame.display.toggle_fullscreen()
    main()

