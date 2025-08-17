import requests
import toml

file_path = "./velox.toml"

plugins = {
    "appLogger": {"owner": "roadrunner-server", "repo": "app-logger"},
    "logger": {"owner": "roadrunner-server", "repo": "logger"},
    "lock": {"owner": "roadrunner-server", "repo": "lock"},
    "rpc": {"owner": "roadrunner-server", "repo": "rpc"},
    "centrifuge": {"owner": "roadrunner-server", "repo": "centrifuge"},
    "temporal": {"owner": "temporalio", "repo": "roadrunner-temporal"},
    "metrics": {"owner": "roadrunner-server", "repo": "metrics"},
    "otel": {"owner": "roadrunner-server", "repo": "otel"},
    "http": {"owner": "roadrunner-server", "repo": "http"},
    "gzip": {"owner": "roadrunner-server", "repo": "gzip"},
    "prometheus": {"owner": "roadrunner-server", "repo": "prometheus"},
    "headers": {"owner": "roadrunner-server", "repo": "headers"},
    "static": {"owner": "roadrunner-server", "repo": "static"},
    "proxy": {"owner": "roadrunner-server", "repo": "proxy_ip_parser"},
    "send": {"owner": "roadrunner-server", "repo": "send"},
    "server": {"owner": "roadrunner-server", "repo": "server"},
    "service": {"owner": "roadrunner-server", "repo": "service"},
    "jobs": {"owner": "roadrunner-server", "repo": "jobs"},
    "amqp": {"owner": "roadrunner-server", "repo": "amqp"},
    "sqs": {"owner": "roadrunner-server", "repo": "sqs"},
    "beanstalk": {"owner": "roadrunner-server", "repo": "beanstalk"},
    "nats": {"owner": "roadrunner-server", "repo": "nats"},
    "kafka": {"owner": "roadrunner-server", "repo": "kafka"},
    "googlepubsub": {"owner": "roadrunner-server", "repo": "google-pub-sub"},
    "kv": {"owner": "roadrunner-server", "repo": "kv"},
    "boltdb": {"owner": "roadrunner-server", "repo": "boltdb"},
    "memory": {"owner": "roadrunner-server", "repo": "memory"},
    "redis": {"owner": "roadrunner-server", "repo": "redis"},
    "memcached": {"owner": "roadrunner-server", "repo": "memcached"},
    "fileserver": {"owner": "roadrunner-server", "repo": "fileserver"},
    "grpc": {"owner": "roadrunner-server", "repo": "grpc"},
    "status": {"owner": "roadrunner-server", "repo": "status"},
    "tcp": {"owner": "roadrunner-server", "repo": "tcp"},
}

# GitHub API URL template
api_url_template = "https://api.github.com/repos/{owner}/{repo}/releases/latest"


# Function to get the latest release tag
def get_latest_release(owner: str, repo: str) -> str:
    url = api_url_template.format(owner=owner, repo=repo)
    response = requests.get(url)
    if response.status_code == 200:
        return response.json()["tag_name"]
    else:
        raise Exception(f"Failed to fetch the latest release for {owner}/{repo}.")


# Fetch the latest release for each plugin
latest_versions = dict[str, str]()
for plugin, info in plugins.items():
    try:
        latest_versions[plugin] = get_latest_release(info["owner"], info["repo"])
    except Exception as e:
        exit(f"Error: {e}")

# Load the existing velox.toml file
with open(file_path, "r") as file:
    config = toml.load(file)

# Update the plugin versions in the config
for plugin, version in latest_versions.items():
    if version:
        config["plugins"][plugin]["tag"] = version

# Save the updated config back to the velox.toml file
with open(file_path, "w") as file:
    toml.dump(config, file)

print("velox.toml file has been updated with the latest plugin versions.")
