extends Node
# StateStreamer.gd - Captures and sends player state to the backend
# Attach this to the Player (CharacterBody3D) node.

var socket := WebSocketPeer.new()
var url := "ws://localhost:8080/api/v1/ws"
var jwt_token := "eyJ0eXAiOiJKV1QiLCJhbGciOiJIUzUxMiJ9.eyJ1c2VySWQiOiJiMmMzZDRlNS0yMjIyLTRhNWYtOWM4Mi0xZDRlNmI4ZjNhMjIiLCJzdWIiOiJ0ZXN0LXVzZXIiLCJyb2xlIjoiQURNSU4iLCJpYXQiOjE3NzQ5MjY2ODgsImV4cCI6MTc3NTAxMzA4OH0.zyooVNBHC-7IrtqGcLRZwHWjMcRPNAIz_aQNvK6-vMunQJuJDZU5yAVE1349qPb7FYehTllqLhcqgmv-baWBeQ"


var last_update_time := 0.0
var update_interval := 0.05 # 20Hz
var mode := "webrtc" # Options: state_sync, webrtc

# --- WebRTC & Video ---
var rtc_peer := WebRTCPeerConnection.new()
var signaling_socket := WebSocketPeer.new()
var signaling_url := "ws://localhost:8080/api/v1/webrtc/signaling?session_id=player_1"

var fifo_path := "/tmp/godot_pipe"
var ffmpeg_pid := -1
var stream_file : FileAccess

# --- Reconnection control ---
var reconnect_timer := 0.0
var reconnect_interval := 1.0
var max_reconnect_interval := 10.0
var is_connecting := false

@onready var player: CharacterBody3D = get_parent()
@onready var camera: Camera3D = player.get_node("Camera3D")

func _ready():
	_parse_cmd_args()
	print("[StateStreamer] Initializing in mode: ", mode)
	
	# Always connect to the input/state socket
	print("[StateStreamer] Connecting to input hub: ", url)
	_try_connect()

	if mode == "webrtc":
		print("[StateStreamer] Connecting to signaling server: ", signaling_url)
		_connect_signaling()
		_start_ffmpeg()

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

		# Send data based on mode
		last_update_time += delta
		if last_update_time >= update_interval:
			last_update_time = 0.0
			if mode == "state_sync":
				send_player_state()
			elif mode == "webrtc":
				_stream_video_frame()

	# Poll signaling socket if in webrtc mode
	if mode == "webrtc":
		signaling_socket.poll()
		_handle_signaling()

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


func _parse_cmd_args():
	for arg in OS.get_cmdline_args():
		if arg.begins_with("--mode="):
			mode = arg.split("=")[1]
			return

func _connect_signaling():
	signaling_socket.handshake_headers = ["Authorization: Bearer " + jwt_token]
	signaling_socket.connect_to_url(signaling_url)
	
	# Initialize RTC Peer
	rtc_peer.ice_candidate_created.connect(_on_ice_candidate)
	rtc_peer.session_description_created.connect(_on_description_created)

func _on_ice_candidate(mid: String, index: int, candidate: String):
	var msg = {
		"type": "candidate",
		"mid": mid,
		"index": index,
		"candidate": candidate
	}
	signaling_socket.send_text(JSON.stringify(msg))

func _on_description_created(type: String, sdp: String):
	rtc_peer.set_local_description(type, sdp)
	var msg = {"type": type, "sdp": sdp}
	signaling_socket.send_text(JSON.stringify(msg))

func _handle_signaling():
	var state = signaling_socket.get_ready_state()
	if state == WebSocketPeer.STATE_OPEN:
		while signaling_socket.get_available_packet_count():
			var packet = signaling_socket.get_packet()
			var msg_string = packet.get_string_from_utf8()
			var json = JSON.new()
			if json.parse(msg_string) == OK:
				var data = json.get_data()
				if data.has("type"):
					if data.type == "offer":
						rtc_peer.set_remote_description("offer", data.sdp)
						rtc_peer.create_answer()
					elif data.type == "answer":
						rtc_peer.set_remote_description("answer", data.sdp)
					elif data.type == "candidate":
						rtc_peer.add_ice_candidate(data.mid, data.index, data.candidate)

func _start_ffmpeg():
	# 1. Create a FIFO (Named Pipe)
	OS.execute("mkfifo", [fifo_path])
	
	# 2. Launch FFmpeg in the background to consume from the pipe
	# and stream to a target. Here we'll just log or save for demonstration.
	# For actual relay, this would push to a socket or WebRTC relay.
	var args := [
		"-f", "rawvideo",
		"-pixel_format", "rgb24",
		"-video_size", "1280x720",
		"-i", fifo_path,
		"-vcodec", "libx264",
		"-preset", "ultrafast",
		"-tune", "zerolatency",
		"-f", "mpegts",
		"udp://127.0.0.1:1234" # Example destination
	]
	
	ffmpeg_pid = OS.create_process("ffmpeg", args)
	print("[StateStreamer] FFmpeg started with PID: ", ffmpeg_pid)
	
	# 3. Open the pipe for writing
	stream_file = FileAccess.open(fifo_path, FileAccess.WRITE)

func _stream_video_frame():
	# Capture the viewport
	var viewport = get_viewport()
	var texture = viewport.get_texture()
	if not texture:
		return
		
	var img = texture.get_image()
	if not img:
		return
	
	# Resize if necessary to match FFmpeg expectation
	if img.get_size() != Vector2i(1280, 720):
		img.resize(1280, 720)
	
	if stream_file:
		stream_file.store_buffer(img.get_data())
		# Flush or allow to buffer? FIFO will block if full.

func _notification(what):
	if what == NOTIFICATION_WM_CLOSE_REQUEST or what == NOTIFICATION_PREDELETE:
		if ffmpeg_pid != -1:
			OS.kill(ffmpeg_pid)
		if stream_file:
			stream_file.close()
		OS.execute("rm", [fifo_path])

func get_current_animation() -> String:
	if not player.is_on_floor():
		return "jumping"
	if player.velocity.length() > 0.1:
		return "running"
	return "idle"
