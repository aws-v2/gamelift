extends CharacterBody3D

# %gun_model
# $Camera3D/gun_model
# $ vs % 
# while using % you use the name "gun_model" directly without having to include the parent 

func _ready():
	# Link the StateStreamer programmatically
	var streamer_script = load("res://player/state_streamer.gd")
	if streamer_script:
		var streamer = Node.new()
		streamer.set_script(streamer_script)
		streamer.name = "StateStreamer"
		add_child(streamer)
		print("[Player] Linked StateStreamer successfully.")
	else:
		printerr("[Player] Failed to load StateStreamer.gd at res://player/state_streamer.gd")

	#4 Remove the cursor from view when game window comes up, ie capture the mouse
	# Skip mouse capture in headless mode (for VM testing)
	if DisplayServer.get_name() != "headless":
		Input.set_mouse_mode(Input.MOUSE_MODE_CAPTURED)
	else:
		print("[Player] Headless mode detected. Skipping mouse capture.")

func  _unhandled_input(event):
	if event is 	InputEventMouseMotion:
		#1 When the mouse moves right to left ie (event.relative.x) 
		# rotate the player character on the y axis ie(rotation_degrees.y)
		# Multiply it  by a factor to slow down the speed of rotation, 0.1 lower, 0.9 higher
		rotation_degrees.y -=event.relative.x*0.3
		
		#2 when you move the mouse  up and down,ie ($Camera3D.rotation_degrees.x) 
		# the camera rotates along the x axis ie(event.relative.y)
		$Camera3D.rotation_degrees.x -= event.relative.y*0.3
 
		#3 Limiting the range of motion 
		$Camera3D.rotation_degrees.x =clamp(
			$Camera3D.rotation_degrees.x, -20.0, 60.0
		)
		
	#	5  When you want to show the cursor in the game window,  click on escape
	elif event.is_action_pressed("ui_cancel"):
		Input.set_mouse_mode(Input.MOUSE_MODE_VISIBLE)
		
		
func _process(delta):
	const SPEED = 4.5
	
	#6 Why this works
	#In most 3D engines (including Godot):
	#X axis = left/right
	#Z axis = forward/back
	#Y axis = up/down (height)
	#So you're basically saying:
	#“Take my 2D input and place it on the ground plane (XZ plane) in 3D.”

	var input_direction_2D = Input.get_vector("move_left", "move_right", "move_forward", "move_back") #6a (x, y)
	var input_direction_3d = Vector3(
		input_direction_2D.x, 
		0.0, 
		input_direction_2D.y
		) #6b (x,y,z) however y is always 0 since we dont have a key for up and down
		
	var direction =  transform.basis * input_direction_3d
#	7 Update the speed on the x and z axis
	velocity.x =SPEED * direction.x
	velocity.z = SPEED * direction.z
	velocity.y -= 20.0 * delta
	
	if  Input.is_action_just_pressed("jump_up") and is_on_floor():
		velocity.y = 10.0
	elif  Input.is_action_just_released("jump_up") and velocity.y >0:
		velocity.y =3
	
	
	move_and_slide()
	
	if Input.is_action_pressed("shoot") and %Timer.is_stopped():
		shoot_bullet()




func shoot_bullet():
	const BULLET_3D = preload("res://player/bullet_3d.tscn")
	var new_bullet = BULLET_3D.instantiate()
	%Marker3D.add_child(new_bullet)
	
	new_bullet.global_transform = %Marker3D.global_transform
	
	%Timer.start() 
	
