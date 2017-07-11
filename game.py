#!/usr/bin/python

import argparse
import datetime
import time

import pygame
from pygame.locals import *

import next_bus


def expected_short(delta):
    ts = delta.total_seconds()
    minutes, seconds = divmod(ts, 60)
    if ts < 15:
        return 'Due'
    if minutes < 0:
        return 'Gone'
    return '%2d:%02d' % (minutes, seconds)


def write_console(background, buses, nlines, status, shift=0):
    font = pygame.font.Font(None, 50)
    background.fill((255, 255, 255,  255))

    now = datetime.datetime.now()
    for i, b in enumerate(buses[0:nlines]):
        mins_left = expected_short(b['when'] - now)
        s = "%3s %6s" % (b['LineName'], mins_left)
        upd(background, s, i, shift)
    write_status(background, status)
    write_time(background)


def write_time(background):
    font = pygame.font.Font(None, 60)

    s = datetime.datetime.now().strftime('%H:%M:%S')
    text = font.render(s, 1, (10, 10, 10))
    textpos = text.get_rect()
    textpos.right = background.get_rect().w
    textpos.bottom = background.get_rect().h
    background.blit(text, textpos)


def write_status(background, s):
    font = pygame.font.Font(None, 60)

    text = font.render(s, 1, (10, 10, 10))
    textpos = text.get_rect()
    textpos.bottom = background.get_rect().h
    textpos.left = 20
    background.blit(text, textpos)


def upd(background, s, n, shift):
    font = pygame.font.Font(None, 180)
    text = font.render(s, 1, (10, 10, 10))
    textpos = text.get_rect()
    #textpos.centerx = background.get_rect().centerx
    textpos.top = 160*n - shift
    textpos.left = 20
    background.blit(text, textpos)

    # Blit everything to the screen

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
                    screen.blit(background, (0, 0))
                    pygame.display.flip()
                buses.pop(0)
            write_console(background, buses, args.num_busses, status)
            screen.blit(background, (0, 0))
            pygame.display.flip()
            time.sleep(1)



def main():
    parser = argparse.ArgumentParser(description='Display a board of next arrival times for TFL buses')
    parser.add_argument('-r', "--route", help="bus route", type=int, default=102)
    parser.add_argument('-s', "--stop", help="bus stop id", default=74640)
    parser.add_argument('-n', "--num_busses", help="number of busses to report", type=int, default=3)
    parser.add_argument('-d', "--delay", help="seconds to wait in between updates", type=int, default=30)
    parser.add_argument('-t', "--test", help="test failure recover", action="store_true")
    args = parser.parse_args()

    # Initialise screen
    pygame.init()
    screen = pygame.display.set_mode((800,480))
    pygame.display.toggle_fullscreen()
    pygame.display.set_caption('bus')

    # Fill background
    background = pygame.Surface(screen.get_size())
    background = background.convert()
    background.fill((250, 250, 250))

    # Event loop
    main_loop(args, background, screen)
    # while 1:
    #     for event in pygame.event.get():
    #         if event.type == QUIT:
    #             return
    #
    #     upd(background)
    #     screen.blit(background, (0, 0))
    #     pygame.display.flip()


if __name__ == '__main__': main()
