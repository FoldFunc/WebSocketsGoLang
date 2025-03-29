import asyncio
import sys
import requests
import websockets

BASE_URL = "http://192.168.1.19:8080"

def register_user(email, password):
    url = f"{BASE_URL}/register"
    data = {"email": email, "password": password}
    response = requests.post(url, json=data)
    if response.status_code == 201:
        print("Registration successful!")
    else:
        print("Registration failed:", response.text)

def login_user(email, password):
    url = f"{BASE_URL}/login"
    data = {"email": email, "password": password}
    session = requests.Session()
    response = session.post(url, json=data)
    if response.status_code == 200:
        print("Login successful!")
    else:
        print("Login failed:", response.text)
    return session

async def chat_client(user_id, peer_id):
    uri = f"ws://192.168.1.19:8080/sendMessage?user_id={user_id}&peer_id={peer_id}"
    print(f"Connecting to {uri}")
    try:
        async with websockets.connect(uri) as websocket:
            async def receiver():
                while True:
                    try:
                        message = await websocket.recv()
                        print(f"\nPeer: {message}")
                    except Exception as e:
                        print("Connection closed or error:", e)
                        break

            recv_task = asyncio.create_task(receiver())

            print("Type your messages below (type 'quit' to exit):")
            while True:
                msg = await asyncio.get_event_loop().run_in_executor(None, sys.stdin.readline)
                msg = msg.strip()
                if msg.lower() == "quit":
                    print("Exiting chat...")
                    break
                if msg:
                    try:
                        await websocket.send(msg)
                    except Exception as e:
                        print("Error sending message:", e)
                        break

            recv_task.cancel()
    except Exception as e:
        print("Failed to connect:", e)

def main():
    mode = input("Select mode: (R)egister, (L)ogin, (C)hat: ").strip().lower()

    if mode == 'r':
        email = input("Enter email: ")
        password = input("Enter password: ")
        register_user(email, password)
    elif mode == 'l':
        email = input("Enter email: ")
        password = input("Enter password: ")
        session = login_user(email, password)
        # At this point, your session cookie is stored in `session` if needed for further authenticated requests.
        chat_prompt = input("Do you want to start chat now? (y/n): ").strip().lower()
        if chat_prompt == 'y':
            user_id = input("Enter your user_id: ")
            peer_id = input("Enter peer user_id: ")
            asyncio.run(chat_client(user_id, peer_id))
    elif mode == 'c':
        # Chat mode assumes you've already registered and know your user_id
        user_id = input("Enter your user_id: ")
        peer_id = input("Enter peer user_id: ")
        asyncio.run(chat_client(user_id, peer_id))
    else:
        print("Invalid mode selected.")

if __name__ == "__main__":
    main()

