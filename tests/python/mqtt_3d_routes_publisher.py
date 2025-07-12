#!/usr/bin/env python3

import time
import math
import json
import paho.mqtt.client as mqtt

# MQTT broker settings
BROKER = 'localhost'
PORT = 1883
TOPIC = 'realtime/3dpoints'

# Generate a circular path point
def generate_point(id: int, radius: float, height: float, angle: float) -> dict:
    x = radius * math.cos(angle)
    y = radius * math.sin(angle)
    z = height
    return {"id": id, "x": x, "y": y, "z": z}

# Main loop: publish points every second
def main():
    client = mqtt.Client()
    client.connect(BROKER, PORT, 60)

    start_time = time.time()
    period = 60  # seconds per full rotation

    try:
        while True:
            t = time.time() - start_time
            # Angle for each route (one full turn per `period` seconds)
            angle = 2 * math.pi * (t % period) / period

            # Define points for each route
            points = [
                generate_point(101, 100, 50, angle),
                generate_point(102, 300, 20, angle)
            ]

            # Construct message
            message = {
                "timestamp": int(t),
                "points": points
            }
            payload = json.dumps(message)

            # Publish to MQTT
            client.publish(TOPIC, payload)
            print(f"Published to {TOPIC}: {payload}")

            time.sleep(1)

    except KeyboardInterrupt:
        print("Interrupted by user, exiting...")
        client.disconnect()

if __name__ == '__main__':
    main()
