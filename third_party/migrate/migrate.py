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
    the_id = 1
    for key in keys:
        # Construct the data payload for the POST request
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

        the_id = the_id + 1

        # Make the POST request to the second API
        try:
            request = Request(second_api_url, method='POST', headers=second_headers)
            request.add_header('Content-Type', 'application/json')
            with urlopen(request, data=json.dumps(payload).encode('utf-8')) as response:
                # Process the second API response as needed
                print(f"Response for name={payload['name']}")
        except HTTPError as e:
            print(f"HTTPError for name={payload['name']}: {e}")
            print(f"Response content: {e.read().decode('utf-8')}")
        except URLError as e:
            print(f"URLError for name={payload['name']}: {e}")


# Main script
oldIP = input("Enter the old server IP: ")
oldPort = input("Enter the old server http port: ")
oldToken = input("Enter the old server token: ")
oldHeaders = {'Authorization': f'Bearer {oldToken}'}
oldKeys = get_keys("http://" + oldIP + ":" + oldPort + "/v1/keys", oldHeaders)

if oldKeys:
    newIP = input("Enter the new server IP: ")
    newPort = input("Enter the new server http port: ")
    newPassword = input("Enter the new server password: ")
    newHeaders = {'Authorization': f'Bearer {newPassword}'}
    process_keys(oldKeys, "http://" + newIP + ":" + newPort + "/v1/users", newHeaders)
