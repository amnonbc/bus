from nose.tools import *

import next_bus
import say_bus


def test_int2bus():
    eq_('1 O 2', say_bus.int2bus(102))

# from TFL doc
field_list = ['StopCode1', 'StopPointName', 'LineName', 'DestinationText', 'EstimatedTime', 'MessageUUID',
              'MessageText', 'MessagePriority', 'MessageType', 'ExpireTime']

response = [
    '[4,"1.0",1334925465143]',
    '[1,"Green Park Station","52053","14","Warren Street",2,1334927247004]',
    '[1,"Green Park Station","52053","22","Piccadilly Cir",1,1334926994196]',
    '[1,"Green Park Station","52053","14","Warren Street",4,1334926832021]',
    '[1,"Green Park Station","52053","22","Piccadilly Cir",5,1334926844473]',
    '[1,"Green Park Station","52053","14","Warren Street",6,1334927176525]'
    ]



def test_parse_bus_response():
    print field_list
    res = next_bus._parse_bus_response(field_list, next_bus.BUS_PREDICTION, response)
    eq_('14', res[0]['LineName'])
    eq_("Green Park Station", res[0]["StopCode1"])
    eq_(2, res[0]["EstimatedTime"])

def test_sort_return_fields():
    f = ['LineName', 'StopCode1']
    next_bus._sort_to_tfl_order(f)
    eq_(['StopCode1', 'LineName'], f)