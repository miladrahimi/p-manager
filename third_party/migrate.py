import json
from urllib.error import URLError, HTTPError
from urllib.request import Request, urlopen


# Function to make the initial API call
def get_keys(api_url, headers):
    try:
        request = Request(api_url, headers=headers)
        with urlopen(request) as response:
            return json.load(response)
    except HTTPError as e:
        print(f"HTTPError: {e}")
        return None
    except URLError as e:
        print(f"URLError: {e}")
        return None


# Function to process each item in the response array and make a POST request to the second API
def process_keys(keys, second_api_url, second_headers):
    the_id = 0
    for key in keys:
        the_id = the_id + 1
        payload = {
            "id": the_id,
            "identity": key.get("code"),
            "method": key.get("cipher"),
            "password": key.get("secret"),
            "name": key.get("name"),
            "quota": int(key.get("quota") / 1000),
            "created_at": key.get("created_at"),
            "enabled": key.get("enabled"),
            "used": key.get("used") / 1000,
            "used_bytes": key.get("used") * 1000 * 1000,
        }

        try:
            request = Request(second_api_url, method='POST', headers=second_headers)
            request.add_header('Content-Type', 'application/json')
            with urlopen(request, data=json.dumps(payload).encode('utf-8')):
                print(f"OK for name={payload['name']}")
        except HTTPError as e:
            print(f"HTTPError for name={payload['name']}: {e}")
            print(f"Response content: {e.read().decode('utf-8')}")
        except URLError as e:
            print(f"URLError for name={payload['name']}: {e}")


old_ip = input("Enter the old server IP: ")
old_port = input("Enter the old server http port (examples: 80 and 8080): ")
old_token = input("Enter the old server token: ")

if not old_ip.startswith("http"):
    old_ip = "http://" + old_ip
if old_ip.endswith("/"):
    old_ip = old_ip[:-1]
old_headers = {'Authorization': f'Bearer {old_token}'}
old_keys = get_keys(old_ip + ":" + old_port + "/v1/keys", old_headers)

if old_keys:
    new_ip = input("Enter the new server IP: ")
    new_port = input("Enter the new server http port (examples: 80 and 8080): ")
    new_password = input("Enter the new server password: ")

    if not new_ip.startswith("http"):
        new_ip = "http://" + new_ip
    if new_ip.endswith("/"):
        new_ip = new_ip[:-1]
    new_headers = {'Authorization': f'Bearer {new_password}'}
    process_keys(old_keys, new_ip + ":" + new_port + "/v1/users", new_headers)
