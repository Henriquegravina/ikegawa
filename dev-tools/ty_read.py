#!/usr/bin/env python3
"""
Read measurements from a Yokogawa TY700/TY500-series DMM (TY710/TY720/TY530)
via the 92015 USB communication adapter.

Protocol reference: Yokogawa IM92015-02E ("92015 Communication Package for
Digital Multimeters" manual).

Serial settings (fixed by the instrument): 9600 bps, 8N, 2 stop bits,
no handshake, <CR><LF> delimiter.

Usage:
    python3 ty_read.py               # single reading
    python3 ty_read.py --stream      # continuous readings until Ctrl+C
    python3 ty_read.py --port /dev/ttyUSB0
"""
import argparse
import re
import sys
import time

import serial

RESP_RE = re.compile(
    r"^RR,(?P<status>[NOB]),(?P<sign>[+-])(?P<value>\d+\.?\d*)"
    r"(?P<aux>[a-zA-Z]?)(?P<unit>[A-Za-z]+?)(?P<func>DC|AC|[A-Za-z]{2})"
    r"(?P<checksum>[0-9A-Fa-f])$"
)

STATUS_MAP = {"N": "Normal", "O": "Over-range", "B": "Low battery"}


def open_port(port: str) -> serial.Serial:
    return serial.Serial(
        port, baudrate=9600, bytesize=8, parity="N", stopbits=2,
        timeout=2, write_timeout=2,
    )


def parse(line: str):
    m = RESP_RE.match(line)
    if not m:
        return None
    d = m.groupdict()
    value = float(d["sign"] + d["value"])
    return {
        "status": STATUS_MAP.get(d["status"], d["status"]),
        "value": value,
        "aux_unit": d["aux"],
        "unit": d["unit"],
        "function": d["func"],
        "text": f"{value}{d['aux']}{d['unit']} {d['func']}",
    }


def single_reading(ser: serial.Serial):
    time.sleep(0.2)  # let the adapter settle right after opening the port
    for attempt in range(3):
        ser.reset_input_buffer()
        ser.write(b"RR,1\r\n")
        ser.flush()
        line = ser.readline().decode("ascii", errors="replace").strip()
        if line.startswith("RR,"):
            return line
        time.sleep(0.2)
    return line


def stream(ser: serial.Serial):
    ser.reset_input_buffer()
    ser.write(b"RR,2\r\n")
    ser.flush()
    try:
        while True:
            line = ser.readline().decode("ascii", errors="replace").strip()
            if not line:
                continue
            parsed = parse(line)
            ts = time.strftime("%H:%M:%S")
            if parsed:
                print(f"{ts}  {parsed['text']:>16s}  ({parsed['status']})", flush=True)
            else:
                print(f"{ts}  raw: {line!r}", flush=True)
    except KeyboardInterrupt:
        pass
    finally:
        ser.write(b"RR,0\r\n")  # stop streaming
        ser.flush()


def main():
    ap = argparse.ArgumentParser(description=__doc__)
    ap.add_argument("--port", default="/dev/ttyUSB0")
    ap.add_argument("--stream", action="store_true", help="continuous readings")
    args = ap.parse_args()

    ser = open_port(args.port)
    try:
        if args.stream:
            print("Streaming (Ctrl+C to stop)...")
            stream(ser)
        else:
            line = single_reading(ser)
            parsed = parse(line)
            if parsed:
                print(f"{parsed['text']}  ({parsed['status']})")
            else:
                print(f"raw: {line!r}", file=sys.stderr)
                sys.exit(1)
    finally:
        ser.close()


if __name__ == "__main__":
    main()
