import asyncio
import goodwe
import json
import os


async def get_runtime_data():
    ip_address = os.getenv('INVERTER_IP')

    inverter = await goodwe.connect(ip_address)

    while True:
        command = input()
        try:
            match command:
                case "get_sensors":
                    sensorList = []
                    runtime_data = await inverter.read_runtime_data()

                    for sensor in inverter.sensors():
                        if sensor.id_ in runtime_data and sensor.id_ != "timestamp":
                            sensorList.append({"Id": sensor.id_, "Name": sensor.name, "Value": str(
                                runtime_data[sensor.id_]), "Unit": sensor.unit})

                    print(json.dumps(sensorList))
                case _:
                    print("Unknown command", command)
        except Exception as e:
            print("Exception occurred", type(e))


asyncio.run(get_runtime_data())
