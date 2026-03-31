extends Node
# StateStreamer.gd - Captures and sends player state to the backend
# Attach this to the Player (CharacterBody3D) node.

var socket := WebSocketPeer.new()
var url := "ws://localhost:8080/api/v1/ws"
var jwt_token := "eyJ0eXAiOiJKV1QiLCJhbGciOiJIUzUxMiJ9.eyJ1c2VySWQiOiJiMmMzZDRlNS0yMjIyLTRhNWYtOWM4Mi0xZDRlNmI4ZjNhMjIiLCJzdWIiOiJ0ZXN0LXVzZXIiLCJyb2xlIjoiQURNSU4iLCJpYXQiOjE3NzQ5MjY2ODgsImV4cCI6MTc3NTAxMzA4OH0.zyooVNBHC-7IrtqGcLRZwHWjMcRPNAIz_aQNvK6-vMunQJuJDZU5yAVE1349qPb7FYehTllqLhcqgmv-baWBeQ"


var last_update_time := 0.0
var update_interval := 0.05 # 20Hz

# --- Reconnection control ---
var reconnect_timer := 0.0
var reconnect_interval := 1.0
var max_reconnect_interval := 10.0
var is_connecting := false

@onready var player: CharacterBody3D = get_parent()
@onready var camera: Camera3D = player.get_node("Camera3D")

func _ready():
	print("[StateStreamer] Connecting to ", url)
	_try_connect()

func _process(delta):
	socket.poll()
	var state = socket.get_ready_state()

	if state == WebSocketPeer.STATE_OPEN:
		# Reset reconnect logic on success
		reconnect_timer = 0.0
		reconnect_interval = 1.0
		is_connecting = false

		# Handle incoming messages
		while socket.get_available_packet_count():
			var packet = socket.get_packet()
			var msg = packet.get_string_from_utf8()
			_handle_incoming_message(msg)

		# Send player state at 20Hz
		last_update_time += delta
		if last_update_time >= update_interval:
			last_update_time = 0.0
			send_player_state()

	elif state == WebSocketPeer.STATE_CLOSED:
		reconnect_timer += delta

		if reconnect_timer >= reconnect_interval:
			reconnect_timer = 0.0
			var code = socket.get_close_code()
			var reason = socket.get_close_reason()
			if code != -1:
				print("[StateStreamer] Connection closed (code: ", code, ", reason: '", reason, "'). Reconnecting...")
			else:
				print("[StateStreamer] Attempting reconnect...")
			_try_connect()

	elif state == WebSocketPeer.STATE_CONNECTING:
		# Do nothing, just wait
		pass

	elif state == WebSocketPeer.STATE_CLOSING:
		# Optional: wait for full close
		pass


func _try_connect():
	# If we are in the middle of a connection attempt, don't start another one.
	# However, if the socket is actually closed, we should allow a new attempt.
	if is_connecting and socket.get_ready_state() != WebSocketPeer.STATE_CLOSED:
		return

	is_connecting = true
	
	# Always start with a fresh socket instance for reconnection
	if socket.get_ready_state() != WebSocketPeer.STATE_CLOSED:
		socket.close()
	socket = WebSocketPeer.new()

	socket.handshake_headers = ["Authorization: Bearer " + jwt_token]
	var err = socket.connect_to_url(url)
	if err != OK:
		printerr("[StateStreamer] Connect error: ", err)
		is_connecting = false

		# Exponential backoff
		reconnect_interval = min(reconnect_interval * 2.0, max_reconnect_interval)


# --- INPUT RELAY LOGIC ---

func _handle_incoming_message(json_string: String):
	var json = JSON.new()
	var error = json.parse(json_string)
	if error != OK:
		return
		
	var data = json.get_data()
	if typeof(data) != TYPE_DICTIONARY:
		return
		
	var type = data.get("type", "")
	var key = data.get("key", "")

	if type == "keydown":
		_process_key_event(key, true)
	elif type == "keyup":
		_process_key_event(key, false)


func _process_key_event(key: String, is_pressed: bool):
	var action = _map_key_to_action(key)

	if action != "":
		if is_pressed:
			Input.action_press(action)
			print("[StateStreamer] Action Pressed: ", action, " (Key: ", key, ")")
		else:
			Input.action_release(action)
			print("[StateStreamer] Action Released: ", action, " (Key: ", key, ")")


func _map_key_to_action(key: String) -> String:
	match key.to_lower():
		"w", "arrowup":
			return "move_forward"
		"s", "arrowdown":
			return "move_back"
		"a", "arrowleft":
			return "move_left"
		"d", "arrowright":
			return "move_right"
		" ": # Space
			return "jump_up"
		"f", "enter":
			return "shoot"
	return ""


# --- STATE SENDING ---

func send_player_state():
	if not player:
		return

	var payload = {
		"id": "player_1",
		"x": player.global_position.x,
		"y": player.global_position.y,
		"z": player.global_position.z,
		"yaw": player.rotation.y,
		"pitch": camera.rotation.x,
		"vx": player.velocity.x,
		"vy": player.velocity.y,
		"vz": player.velocity.z,
		"anim": get_current_animation(),
		"is_on_floor": player.is_on_floor(),
		"health": 100 # Placeholder
	}

	var json_data = JSON.stringify(payload)
	var err = socket.send_text(json_data)

	if err == OK:
		print("[StateStreamer] Sent: ", json_data)
	else:
		printerr("[StateStreamer] Send error: ", err)


func get_current_animation() -> String:
	if not player.is_on_floor():
		return "jumping"
	if player.velocity.length() > 0.1:
		return "running"
	return "idle"
