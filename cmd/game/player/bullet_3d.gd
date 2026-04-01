extends Area3D

const SPEED = 20.0
const RANGE = 40.0

var travelled_distance =0.0
# Called when the node enters the scene tree for the first time.
func _ready():
	pass # Replace with function body.


# Called every frame. 'delta' is the elapsed time since the previous frame.
func _process(delta):
	position += -transform.basis.z * SPEED * delta
	travelled_distance += delta*RANGE
	if travelled_distance >= RANGE:
		queue_free()
