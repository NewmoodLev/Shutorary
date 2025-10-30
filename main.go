package main

import (
	"fmt"
	"math"
	"math/rand"
	"os"
	"time"

	rl "github.com/gen2brain/raylib-go/raylib"
)

// Game States
type GameState int

const (
	StateMenu GameState = iota
	StateSettings
	StatePlaying
	StatePaused
	StateUpgrade
	StateGameOver
)

// Stage Types - เปลี่ยนทุก 20 level
type StageType int

const (
	StageBasic StageType = iota
	StageMaze
	StageHazard
	StageArena
)

// Data structures
type Player struct {
	position rl.Vector3
	angle    float32
	health   int
	stats    PlayerStats
	lastShot float32
	skills   []Skill
	color    rl.Color
	id       int
	model    rl.Model

	// Added: model scale and yaw offset (degrees) - ปรับค่าได้จากโค้ดตรงนี้
	modelScale        float32
	modelYawOffsetDeg float32

	// ใหม่: fields สำหรับการเคลื่อนที่และการกระเด้ง
	bobHeight float32 // ความสูงการกระเด้งขึ้นลง
	bobSpeed  float32 // ความเร็วการกระเด้ง
	bobTime   float32 // เวลาสำหรับคำนวณการกระเด้ง
	tiltAngle float32 // มุมเอียงซ้าย-ขวา

	// เพิ่มฟิลด์สำหรับแอนิเมชั่น
	animTime    float32
	isMoving    bool
	walkBobAmp  float32 // ความสูงของการกระเด้ง
	walkBobFreq float32 // ความเร็วของการกระเด้ง
}

type Enemy struct {
	position  rl.Vector3
	velocity  rl.Vector3
	active    bool
	health    int
	maxHealth int
	size      float32
	color     rl.Color
	isBoss    bool
	model     rl.Model
	hasModel  bool

	// Added: per-enemy model scale and yaw offset (set on spawn)
	modelScale        float32
	modelYawOffsetDeg float32
}

type Bullet struct {
	position rl.Vector3
	velocity rl.Vector3
	active   bool
	damage   int
	playerId int
}

type Particle struct {
	position rl.Vector3
	velocity rl.Vector3
	lifetime float32
	active   bool
	color    rl.Color
}

type PowerUp struct {
	position rl.Vector3
	active   bool
	pType    int
	rotation float32
}

type Obstacle struct {
	position rl.Vector3
	size     rl.Vector3
	active   bool
	obsType  int // 0=wall, 1=hazard
}

type Skill struct {
	name        string
	cooldown    float32
	maxCooldown float32
	ready       bool
}

type PlayerStats struct {
	maxHealth  int
	damage     int
	speed      float32
	fireRate   float32
	critChance float32
	statPoints int
}

type SoundSystem struct {
	shoot       rl.Sound
	explosion   rl.Sound
	hit         rl.Sound
	powerup     rl.Sound
	skill       rl.Sound
	boss        rl.Sound
	menuBGM     rl.Music
	gameBGM     rl.Music
	enabled     bool
	menuPlaying bool
	gamePlaying bool
}

type Settings struct {
	soundEnabled bool
	musicEnabled bool
	soundVolume  float32
	musicVolume  float32
	difficulty   int // 0=Easy, 1=Normal, 2=Hard
}

// Constants
const (
	screenWidth   = 1800
	screenHeight  = 1028
	maxEnemies    = 80  // เพิ่มจำนวน
	maxBullets    = 20  // เพิ่มจำนวน
	maxParticles  = 200 // ลดลงเพื่อ performance
	maxPowerUps   = 5
	maxObstacles  = 30
	stageInterval = 10 // เปลี่ยน stage ทุก 20 level
)

// Default model scale / yaw offsets (ปรับค่าเพื่อเปลี่ยนสเกลจากโค้ดได้)
var (
	DefaultPlayerScale        float32 = 2.0
	DefaultPlayer2Scale       float32 = 2.0
	DefaultPlayerYawOffsetDeg float32 = -90.0

	DefaultEnemyScaleFactor  float32 = 6.0 // คูณกับ enemy.size
	DefaultEnemyYawOffsetDeg float32 = 0.0

	DefaultBossScaleFactor  float32 = 4.2 // คูณกับ boss size
	DefaultBossYawOffsetDeg float32 = 0.0
)

// Game state
type Game struct {
	state             GameState
	camera            rl.Camera3D
	players           []Player
	enemies           []Enemy
	bullets           []Bullet
	particles         []Particle
	powerUps          []PowerUp
	obstacles         []Obstacle
	sounds            SoundSystem
	settings          Settings
	score             int
	highScore         int
	level             int
	spawnTimer        float32
	spawnInterval     float32
	enemiesKilled     int
	gameTime          float32
	bossActive        bool
	bossSpawned       bool
	upgradeChoice     int
	coopMode          bool
	menuSelection     int
	settingsSelection int
	currentStage      StageType
	playerModel       rl.Model
	enemyModel        rl.Model
	bossModel         rl.Model
	modelsLoaded      bool
}

func NewGame() *Game {
	g := &Game{
		state:             StateMenu,
		enemies:           make([]Enemy, maxEnemies),
		bullets:           make([]Bullet, maxBullets),
		particles:         make([]Particle, maxParticles),
		powerUps:          make([]PowerUp, maxPowerUps),
		obstacles:         make([]Obstacle, maxObstacles),
		menuSelection:     0,
		settingsSelection: 0,
		currentStage:      StageBasic,
		settings: Settings{
			soundEnabled: true,
			musicEnabled: true,
			soundVolume:  0.5,
			musicVolume:  0.3,
			difficulty:   1,
		},
	}

	// Isometric camera setup
	g.camera = rl.Camera3D{
		Position:   rl.NewVector3(25, 25, 25),
		Target:     rl.NewVector3(0, 0, 0),
		Up:         rl.NewVector3(0, 1, 0),
		Fovy:       45,
		Projection: rl.CameraPerspective,
	}

	// Load sounds and models
	g.loadSounds()
	g.loadModels()

	return g
}

func fileExists(filename string) bool {
	_, err := os.Stat(filename)
	return err == nil
}

func (g *Game) loadModels() {
	// สร้างโฟลเดอร์ assets ถ้ายังไม่มี
	os.MkdirAll("assets/models", os.ModePerm)

	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Warning: Could not load 3D models, using basic shapes")
			g.modelsLoaded = false
		}
	}()

	// พยายามโหลด models - ลองหลายสกุลไฟล์ (ลำดับ: GLB > GLTF > FBX > OBJ)
	// Player model
	playerLoaded := false
	if fileExists("assets/models/player.glb") {
		g.playerModel = rl.LoadModel("assets/models/player.glb")
		playerLoaded = true
		g.modelsLoaded = true
		fmt.Println("✓ Loaded: player.glb")
	} else if fileExists("assets/models/player.gltf") {
		g.playerModel = rl.LoadModel("assets/models/player.gltf")
		playerLoaded = true
		g.modelsLoaded = true
		fmt.Println("✓ Loaded: player.gltf")
	} else if fileExists("assets/models/player.fbx") {
		g.playerModel = rl.LoadModel("assets/models/player.fbx")
		playerLoaded = true
		g.modelsLoaded = true
		fmt.Println("✓ Loaded: player.fbx")
	} else if fileExists("assets/models/player.obj") {
		g.playerModel = rl.LoadModel("assets/models/player.obj")
		playerLoaded = true
		g.modelsLoaded = true
		fmt.Println("✓ Loaded: player.obj")
	}

	// Enemy model
	enemyLoaded := false
	if fileExists("assets/models/enemy.glb") {
		g.enemyModel = rl.LoadModel("assets/models/enemy.glb")
		enemyLoaded = true
		fmt.Println("✓ Loaded: enemy.glb")
	} else if fileExists("assets/models/enemy.gltf") {
		g.enemyModel = rl.LoadModel("assets/models/enemy.gltf")
		enemyLoaded = true
		fmt.Println("✓ Loaded: enemy.gltf")
	} else if fileExists("assets/models/enemy.fbx") {
		g.enemyModel = rl.LoadModel("assets/models/enemy.fbx")
		enemyLoaded = true
		fmt.Println("✓ Loaded: enemy.fbx")
	} else if fileExists("assets/models/enemy.obj") {
		g.enemyModel = rl.LoadModel("assets/models/enemy.obj")
		enemyLoaded = true
		fmt.Println("✓ Loaded: enemy.obj")
	}

	// Boss model
	bossLoaded := false
	if fileExists("assets/models/boss.glb") {
		g.bossModel = rl.LoadModel("assets/models/boss.glb")
		bossLoaded = true
		fmt.Println("✓ Loaded: boss.glb")
	} else if fileExists("assets/models/boss.gltf") {
		g.bossModel = rl.LoadModel("assets/models/boss.gltf")
		bossLoaded = true
		fmt.Println("✓ Loaded: boss.gltf")
	} else if fileExists("assets/models/boss.fbx") {
		g.bossModel = rl.LoadModel("assets/models/boss.fbx")
		bossLoaded = true
		fmt.Println("✓ Loaded: boss.fbx")
	} else if fileExists("assets/models/boss.obj") {
		g.bossModel = rl.LoadModel("assets/models/boss.obj")
		bossLoaded = true
		fmt.Println("✓ Loaded: boss.obj")
	}

	if !playerLoaded && !enemyLoaded && !bossLoaded {
		fmt.Println("⚠ No models found - using basic cube shapes")
		g.modelsLoaded = false
	} else {
		fmt.Printf("📦 Models Status: Player=%v, Enemy=%v, Boss=%v\n", playerLoaded, enemyLoaded, bossLoaded)
	}
}

func (g *Game) createPlayer(id int, pos rl.Vector3, color rl.Color) Player {
	stats := PlayerStats{
		maxHealth:  100,
		damage:     1,
		speed:      12.0,
		fireRate:   0.15,
		critChance: 0.05,
		statPoints: 0,
	}

	skills := []Skill{
		{name: "Explosion", cooldown: 0, maxCooldown: 8.0, ready: true},
		{name: "Radial Shot", cooldown: 0, maxCooldown: 10.0, ready: true},
		{name: "Energy Shield", cooldown: 0, maxCooldown: 15.0, ready: true},
	}

	// Choose per-player default scale (player 2 smaller by default)
	scale := DefaultPlayerScale
	if id == 1 {
		scale = DefaultPlayer2Scale
	}

	return Player{
		position: pos,
		angle:    0,
		health:   stats.maxHealth,
		stats:    stats,
		lastShot: 0,
		skills:   skills,
		color:    color,
		id:       id,
		model:    g.playerModel,

		// Default scale and yaw offset (แก้ค่าที่นี่ถ้าต้องการ)
		modelScale:        scale,
		modelYawOffsetDeg: DefaultPlayerYawOffsetDeg,

		// ใหม่: ค่าเริ่มต้นสำหรับการกระเด้งและการเอียง
		bobHeight: 0.15, // ความสูงการกระเด้ง
		bobSpeed:  8.0,  // ความเร็วการกระเด้ง
		bobTime:   0,
		tiltAngle: 0,

		// ค่าเริ่มต้นสำหรับแอนิเมชั่น
		animTime:    0,
		isMoving:    false,
		walkBobAmp:  0.15, // ปรับความสูงของการกระเด้ง
		walkBobFreq: 8.0,  // ปรับความเร็วของการกระเด้ง
	}
}

func (g *Game) loadSounds() {
	rl.InitAudioDevice()

	if rl.IsAudioDeviceReady() {
		g.sounds.enabled = true

		defer func() {
			if r := recover(); r != nil {
				fmt.Println("Warning: Some sound files not found, continuing without sound")
				g.sounds.enabled = false
			}
		}()

		os.MkdirAll("assets/sounds", os.ModePerm)

		// โหลดเสียงเอฟเฟกต์
		if fileExists("assets/sounds/shoot.wav") {
			g.sounds.shoot = rl.LoadSound("assets/sounds/shoot.wav")
		}
		if fileExists("assets/sounds/explosion.wav") {
			g.sounds.explosion = rl.LoadSound("assets/sounds/explosion.wav")
		}
		if fileExists("assets/sounds/hit.wav") {
			g.sounds.hit = rl.LoadSound("assets/sounds/hit.wav")
		}
		if fileExists("assets/sounds/powerup.wav") {
			g.sounds.powerup = rl.LoadSound("assets/sounds/powerup.wav")
		}
		if fileExists("assets/sounds/skill.wav") {
			g.sounds.skill = rl.LoadSound("assets/sounds/skill.wav")
		}
		if fileExists("assets/sounds/boss.wav") {
			g.sounds.boss = rl.LoadSound("assets/sounds/boss.wav")
		}

		// โหลดเพลง BGM แยกกัน
		if fileExists("assets/sounds/menu_bgm.mp3") {
			g.sounds.menuBGM = rl.LoadMusicStream("assets/sounds/menu_bgm.mp3")
		}
		if fileExists("assets/sounds/game_bgm.mp3") {
			g.sounds.gameBGM = rl.LoadMusicStream("assets/sounds/game_bgm.mp3")
		}

		g.updateVolume()
	}
}

func (g *Game) updateVolume() {
	if !g.sounds.enabled {
		return
	}

	if g.sounds.menuBGM.CtxType != 0 {
		rl.SetMusicVolume(g.sounds.menuBGM, g.settings.musicVolume)
	}
	if g.sounds.gameBGM.CtxType != 0 {
		rl.SetMusicVolume(g.sounds.gameBGM, g.settings.musicVolume)
	}
}

func (g *Game) updateMusic() {
	if !g.sounds.enabled || !g.settings.musicEnabled {
		return
	}

	// จัดการเพลงตาม state
	if g.state == StateMenu || g.state == StateSettings {
		if !g.sounds.menuPlaying && g.sounds.menuBGM.CtxType != 0 {
			rl.PlayMusicStream(g.sounds.menuBGM)
			g.sounds.menuPlaying = true
		}
		if g.sounds.gamePlaying && g.sounds.gameBGM.CtxType != 0 {
			rl.StopMusicStream(g.sounds.gameBGM)
			g.sounds.gamePlaying = false
		}
		if g.sounds.menuBGM.CtxType != 0 {
			rl.UpdateMusicStream(g.sounds.menuBGM)
		}
	} else if g.state == StatePlaying {
		if !g.sounds.gamePlaying && g.sounds.gameBGM.CtxType != 0 {
			rl.PlayMusicStream(g.sounds.gameBGM)
			g.sounds.gamePlaying = true
		}
		if g.sounds.menuPlaying && g.sounds.menuBGM.CtxType != 0 {
			rl.StopMusicStream(g.sounds.menuBGM)
			g.sounds.menuPlaying = false
		}
		if g.sounds.gameBGM.CtxType != 0 {
			rl.UpdateMusicStream(g.sounds.gameBGM)
		}
	}
}

func (g *Game) playSound(sound rl.Sound) {
	if g.sounds.enabled && g.settings.soundEnabled && sound.FrameCount > 0 {
		rl.SetSoundVolume(sound, g.settings.soundVolume)
		rl.PlaySound(sound)
	}
}

func (g *Game) StartGame(coopMode bool) {
	g.coopMode = coopMode
	g.state = StatePlaying

	if coopMode {
		g.players = make([]Player, 2)
		g.players[0] = g.createPlayer(0, rl.NewVector3(-3, 0.5, 0), rl.Blue)
		g.players[1] = g.createPlayer(1, rl.NewVector3(3, 0.5, 0), rl.Green)
	} else {
		g.players = make([]Player, 1)
		g.players[0] = g.createPlayer(0, rl.NewVector3(0, 0.5, 0), rl.Blue)
	}

	g.ResetGame()
}

func (g *Game) ResetGame() {
	for i := range g.players {
		if g.coopMode {
			if i == 0 {
				g.players[i].position = rl.NewVector3(-3, 0.5, 0)
			} else {
				g.players[i].position = rl.NewVector3(3, 0.5, 0)
			}
		} else {
			g.players[i].position = rl.NewVector3(0, 0.5, 0)
		}
		g.players[i].angle = 0
		g.players[i].stats = PlayerStats{
			maxHealth:  100,
			damage:     1,
			speed:      12.0,
			fireRate:   0.15,
			critChance: 0.05,
			statPoints: 0,
		}
		g.players[i].health = g.players[i].stats.maxHealth
		g.players[i].lastShot = 0

		for j := range g.players[i].skills {
			g.players[i].skills[j].cooldown = 0
			g.players[i].skills[j].ready = true
		}
	}

	g.score = 0
	g.level = 1
	g.spawnTimer = 0
	g.spawnInterval = 1.5
	g.enemiesKilled = 0
	g.gameTime = 0
	g.bossActive = false
	g.bossSpawned = false
	g.upgradeChoice = -1
	g.currentStage = StageBasic

	// Apply difficulty
	switch g.settings.difficulty {
	case 0: // Easy
		g.spawnInterval = 2.0
		for i := range g.players {
			g.players[i].stats.maxHealth = 150
			g.players[i].health = 150
		}
	case 2: // Hard
		g.spawnInterval = 1.0
		for i := range g.players {
			g.players[i].stats.maxHealth = 75
			g.players[i].health = 75
		}
	}

	for i := range g.enemies {
		g.enemies[i].active = false
	}
	for i := range g.bullets {
		g.bullets[i].active = false
	}
	for i := range g.particles {
		g.particles[i].active = false
	}
	for i := range g.powerUps {
		g.powerUps[i].active = false
	}
	for i := range g.obstacles {
		g.obstacles[i].active = false
	}

	g.GenerateStage()
}

func (g *Game) GenerateStage() {
	// ล้าง obstacles เก่า
	for i := range g.obstacles {
		g.obstacles[i].active = false
	}

	// กำหนด stage type ตาม level
	stageNum := (g.level - 1) / stageInterval
	g.currentStage = StageType(stageNum % 4)

	switch g.currentStage {
	case StageMaze:
		g.GenerateMaze()
	case StageHazard:
		g.GenerateHazards()
	case StageArena:
		g.GenerateArena()
	}
}

func (g *Game) GenerateMaze() {
	// สร้างกำแพงแบบ maze
	obsIndex := 0
	for i := -20; i <= 20; i += 10 {
		if obsIndex >= maxObstacles {
			break
		}
		g.obstacles[obsIndex] = Obstacle{
			position: rl.NewVector3(float32(i), 1, 0),
			size:     rl.NewVector3(2, 3, 15),
			active:   true,
			obsType:  0,
		}
		obsIndex++
	}

	for i := -15; i <= 15; i += 10 {
		if obsIndex >= maxObstacles {
			break
		}
		g.obstacles[obsIndex] = Obstacle{
			position: rl.NewVector3(0, 1, float32(i)),
			size:     rl.NewVector3(15, 3, 2),
			active:   true,
			obsType:  0,
		}
		obsIndex++
	}
}

func (g *Game) GenerateHazards() {
	// สร้างพื้นที่อันตราย
	obsIndex := 0
	for i := 0; i < 10 && obsIndex < maxObstacles; i++ {
		angle := rand.Float64() * 2 * math.Pi
		distance := 10.0 + rand.Float64()*10

		g.obstacles[obsIndex] = Obstacle{
			position: rl.NewVector3(
				float32(math.Cos(angle)*distance),
				0.5,
				float32(math.Sin(angle)*distance),
			),
			size:    rl.NewVector3(3, 1, 3),
			active:  true,
			obsType: 1, // hazard
		}
		obsIndex++
	}
}

func (g *Game) GenerateArena() {
	// สร้างกำแพงรอบ arena
	obsIndex := 0
	walls := []struct {
		pos  rl.Vector3
		size rl.Vector3
	}{
		{rl.NewVector3(0, 1, -18), rl.NewVector3(30, 3, 2)}, // North
		{rl.NewVector3(0, 1, 18), rl.NewVector3(30, 3, 2)},  // South
		{rl.NewVector3(-18, 1, 0), rl.NewVector3(2, 3, 30)}, // West
		{rl.NewVector3(18, 1, 0), rl.NewVector3(2, 3, 30)},  // East
	}

	for _, wall := range walls {
		if obsIndex >= maxObstacles {
			break
		}
		g.obstacles[obsIndex] = Obstacle{
			position: wall.pos,
			size:     wall.size,
			active:   true,
			obsType:  0,
		}
		obsIndex++
	}
}

func (g *Game) CheckObstacleCollision(pos rl.Vector3, radius float32) bool {
	for i := range g.obstacles {
		if !g.obstacles[i].active {
			continue
		}

		obs := &g.obstacles[i]
		// Simple AABB collision
		if pos.X+radius > obs.position.X-obs.size.X/2 &&
			pos.X-radius < obs.position.X+obs.size.X/2 &&
			pos.Z+radius > obs.position.Z-obs.size.Z/2 &&
			pos.Z-radius < obs.position.Z+obs.size.Z/2 {
			return true
		}
	}
	return false
}

// --- Added: clamp player position to stage/map bounds to prevent leaving map ---
func (g *Game) clampPlayerToStageBounds(player *Player, margin float32) {
	// Default map half-size (plane is 60x60 => half = 30)
	half := float32(30.0) - margin

	// For specific stages adjust the playable area (e.g., arena walls at ±18)
	switch g.currentStage {
	case StageArena:
		half = float32(18.0) - margin
	case StageMaze:
		// Maze walls may be placed inside +/-20 but keep safe margin
		half = float32(28.0) - margin
	case StageHazard:
		// Hazard uses full map but reserve margin
		half = float32(29.0) - margin
	}

	// Clamp X,Z
	if player.position.X < -half {
		player.position.X = -half
	}
	if player.position.X > half {
		player.position.X = half
	}
	if player.position.Z < -half {
		player.position.Z = -half
	}
	if player.position.Z > half {
		player.position.Z = half
	}
}

func (g *Game) ApplyUpgrade(choice int) {
	for i := range g.players {
		switch choice {
		case 0:
			g.players[i].stats.maxHealth += 20
			g.players[i].health = g.players[i].stats.maxHealth
		case 1:
			g.players[i].stats.damage++
		case 2:
			g.players[i].stats.speed += 2.0
		case 3:
			g.players[i].stats.fireRate = float32(math.Max(float64(g.players[i].stats.fireRate-0.02), 0.05))
		case 4:
			g.players[i].stats.critChance = float32(math.Min(float64(g.players[i].stats.critChance+0.05), 0.5))
		}
	}
	g.state = StatePlaying
	g.upgradeChoice = -1
}

func (g *Game) SpawnBoss() {
	for i := range g.enemies {
		if !g.enemies[i].active {
			angle := rand.Float64() * 2 * math.Pi
			distance := 30.0

			bossHealth := 50 + g.level*10
			bossSize := float32(4.0)

			g.enemies[i] = Enemy{
				position: rl.NewVector3(
					float32(math.Cos(angle)*distance),
					1.5,
					float32(math.Sin(angle)*distance),
				),
				velocity:          rl.NewVector3(0, 0, 0),
				health:            bossHealth,
				maxHealth:         bossHealth,
				size:              bossSize,
				active:            true,
				isBoss:            true,
				color:             rl.NewColor(150, 0, 150, 255),
				model:             g.bossModel,
				hasModel:          g.modelsLoaded,
				modelScale:        DefaultBossScaleFactor * bossSize,
				modelYawOffsetDeg: DefaultBossYawOffsetDeg,
			}

			g.bossActive = true
			g.bossSpawned = true
			g.playSound(g.sounds.boss)
			break
		}
	}
}

func (g *Game) SpawnEnemy() {
	for i := range g.enemies {
		if !g.enemies[i].active {
			angle := rand.Float64() * 2 * math.Pi
			distance := 25.0 + rand.Float64()*5

			pos := rl.NewVector3(
				float32(math.Cos(angle)*distance),
				0.75,
				float32(math.Sin(angle)*distance),
			)

			// ตรวจสอบว่าไม่ชนกับ obstacle
			if g.CheckObstacleCollision(pos, 1.0) {
				continue
			}

			targetPlayer := g.players[rand.Intn(len(g.players))]
			dx := targetPlayer.position.X - pos.X
			dz := targetPlayer.position.Z - pos.Z
			dist := float32(math.Sqrt(float64(dx*dx + dz*dz)))

			speed := float32(3.0 + rand.Float64()*2 + float64(g.level)*0.5)

			size := 1.0 + rand.Float32()*0.5
			g.enemies[i] = Enemy{
				position: rl.NewVector3(
					pos.X,
					pos.Y,
					pos.Z,
				),
				velocity:          rl.NewVector3(dx/dist*speed, 0, dz/dist*speed),
				health:            1 + (g.level-1)/3,
				maxHealth:         1 + (g.level-1)/3,
				size:              size,
				active:            true,
				isBoss:            false,
				color:             rl.NewColor(uint8(200+rand.Intn(56)), uint8(50-g.level*2), uint8(50-g.level*2), 255),
				model:             g.enemyModel,
				hasModel:          g.modelsLoaded,
				modelScale:        DefaultEnemyScaleFactor * size,
				modelYawOffsetDeg: DefaultEnemyYawOffsetDeg,
			}
			break
		}
	}
}

func (g *Game) ShootBullet(player *Player) {
	now := g.gameTime
	if now-player.lastShot < player.stats.fireRate {
		return
	}

	for i := range g.bullets {
		if !g.bullets[i].active {
			g.bullets[i].position = player.position
			g.bullets[i].position.Y = 1

			dirX := float32(math.Cos(float64(player.angle)))
			dirZ := float32(math.Sin(float64(player.angle)))

			speed := float32(40.0)
			g.bullets[i].velocity = rl.NewVector3(dirX*speed, 0, dirZ*speed)
			g.bullets[i].active = true
			g.bullets[i].playerId = player.id

			damage := player.stats.damage
			if rand.Float32() < player.stats.critChance {
				damage *= 3
			}
			g.bullets[i].damage = damage
			player.lastShot = now
			g.playSound(g.sounds.shoot)
			break
		}
	}
}

func (g *Game) UseSkill(player *Player, skillIndex int) {
	if !player.skills[skillIndex].ready {
		return
	}

	switch skillIndex {
	case 0: // Explosion
		for i := range g.enemies {
			if g.enemies[i].active {
				dx := g.enemies[i].position.X - player.position.X
				dz := g.enemies[i].position.Z - player.position.Z
				dist := math.Sqrt(float64(dx*dx + dz*dz))

				if dist < 10.0 {
					damage := 3 * player.stats.damage
					if g.enemies[i].isBoss {
						damage = 10 * player.stats.damage
					}
					g.enemies[i].health -= damage
					g.CreateExplosion(g.enemies[i].position, rl.Orange, 10)

					if g.enemies[i].health <= 0 {
						g.KillEnemy(i)
					}
				}
			}
		}
		g.CreateExplosion(player.position, rl.Orange, 30)
		g.playSound(g.sounds.skill)

	case 1: // Radial Shot
		for angle := 0.0; angle < 360.0; angle += 30.0 {
			rad := angle * math.Pi / 180.0
			for i := range g.bullets {
				if !g.bullets[i].active {
					g.bullets[i].position = player.position
					g.bullets[i].position.Y = 1
					speed := float32(35.0)
					g.bullets[i].velocity = rl.NewVector3(
						float32(math.Cos(rad))*speed,
						0,
						float32(math.Sin(rad))*speed,
					)
					g.bullets[i].active = true
					g.bullets[i].damage = player.stats.damage
					g.bullets[i].playerId = player.id
					break
				}
			}
		}
		g.playSound(g.sounds.skill)

	case 2: // Energy Shield
		healAmount := 30
		player.health = int(math.Min(float64(player.health+healAmount), float64(player.stats.maxHealth)))
		g.CreateExplosion(player.position, rl.Green, 20)
		g.playSound(g.sounds.skill)
	}

	player.skills[skillIndex].ready = false
	player.skills[skillIndex].cooldown = player.skills[skillIndex].maxCooldown
}

func (g *Game) KillEnemy(index int) {
	g.enemies[index].active = false

	if g.enemies[index].isBoss {
		g.score += 500
		g.bossActive = false
		g.CreateExplosion(g.enemies[index].position, rl.Purple, 50)
		g.level++
		g.bossSpawned = false
		g.playSound(g.sounds.explosion)

		// ตรวจสอบว่าต้องเปลี่ยน stage หรือไม่
		if g.level%stageInterval == 1 {
			g.GenerateStage()
		}

		if g.level%3 == 1 && g.level > 1 {
			g.state = StateUpgrade
		}
	} else {
		g.score += 10 * g.level
		g.playSound(g.sounds.hit)
	}

	g.enemiesKilled++
	g.CreateExplosion(g.enemies[index].position, g.enemies[index].color, 15)
	g.SpawnPowerUp(g.enemies[index].position)

	if !g.enemies[index].isBoss && g.enemiesKilled%20 == 0 && g.level%10 != 0 {
		g.level++
		g.spawnInterval = float32(math.Max(0.5, float64(1.5-float32(g.level)*0.05)))

		// ตรวจสอบว่าต้องเปลี่ยน stage หรือไม่
		if g.level%stageInterval == 1 {
			g.GenerateStage()
		}

		if g.level%3 == 1 && g.level > 1 {
			g.state = StateUpgrade
		}
	}
}

func (g *Game) CreateExplosion(pos rl.Vector3, color rl.Color, count int) {
	// ลด particle เพื่อ performance
	count = int(math.Min(float64(count), 15))

	for j := 0; j < count; j++ {
		for i := range g.particles {
			if !g.particles[i].active {
				angle := rand.Float64() * 2 * math.Pi
				speed := 5.0 + rand.Float64()*10

				g.particles[i].position = pos
				g.particles[i].velocity = rl.NewVector3(
					float32(math.Cos(angle)*speed),
					float32(rand.Float64()*10),
					float32(math.Sin(angle)*speed),
				)
				g.particles[i].lifetime = 0.5 + rand.Float32()*0.5
				g.particles[i].active = true
				g.particles[i].color = color
				break
			}
		}
	}
}

func (g *Game) SpawnPowerUp(pos rl.Vector3) {
	if rand.Float32() > 0.3 {
		return
	}

	for i := range g.powerUps {
		if !g.powerUps[i].active {
			g.powerUps[i].position = pos
			g.powerUps[i].position.Y = 1
			g.powerUps[i].pType = rand.Intn(3)
			g.powerUps[i].active = true
			break
		}
	}
}

func (g *Game) UpdateMenu(dt float32) {
	if rl.IsKeyPressed(rl.KeyUp) {
		g.menuSelection--
		if g.menuSelection < 0 {
			g.menuSelection = 3
		}
	}
	if rl.IsKeyPressed(rl.KeyDown) {
		g.menuSelection++
		if g.menuSelection > 3 {
			g.menuSelection = 0
		}
	}

	if rl.IsKeyPressed(rl.KeyEnter) || rl.IsKeyPressed(rl.KeySpace) {
		switch g.menuSelection {
		case 0:
			g.StartGame(false)
		case 1:
			g.StartGame(true)
		case 2:
			g.state = StateSettings
		case 3:
			os.Exit(0)
		}
	}
}

func (g *Game) UpdateSettings(dt float32) {
	if rl.IsKeyPressed(rl.KeyUp) {
		g.settingsSelection--
		if g.settingsSelection < 0 {
			g.settingsSelection = 5
		}
	}
	if rl.IsKeyPressed(rl.KeyDown) {
		g.settingsSelection++
		if g.settingsSelection > 5 {
			g.settingsSelection = 0
		}
	}

	if rl.IsKeyPressed(rl.KeyLeft) || rl.IsKeyPressed(rl.KeyRight) {
		right := rl.IsKeyPressed(rl.KeyRight)

		switch g.settingsSelection {
		case 0:
			g.settings.soundEnabled = !g.settings.soundEnabled
		case 1:
			g.settings.musicEnabled = !g.settings.musicEnabled
		case 2:
			if right {
				g.settings.soundVolume = float32(math.Min(1.0, float64(g.settings.soundVolume+0.1)))
			} else {
				g.settings.soundVolume = float32(math.Max(0.0, float64(g.settings.soundVolume-0.1)))
			}
		case 3:
			if right {
				g.settings.musicVolume = float32(math.Min(1.0, float64(g.settings.musicVolume+0.1)))
			} else {
				g.settings.musicVolume = float32(math.Max(0.0, float64(g.settings.musicVolume-0.1)))
			}
			g.updateVolume()
		case 4:
			if right {
				g.settings.difficulty++
				if g.settings.difficulty > 2 {
					g.settings.difficulty = 2
				}
			} else {
				g.settings.difficulty--
				if g.settings.difficulty < 0 {
					g.settings.difficulty = 0
				}
			}
		}
	}

	if rl.IsKeyPressed(rl.KeyEscape) || (g.settingsSelection == 5 && (rl.IsKeyPressed(rl.KeyEnter) || rl.IsKeyPressed(rl.KeySpace))) {
		g.state = StateMenu
	}
}

// Update game playing state
func (g *Game) Update(dt float32) {
	// Update music based on state
	g.updateMusic()

	switch g.state {
	case StateMenu:
		g.UpdateMenu(dt)
		return

	case StateSettings:
		g.UpdateSettings(dt)
		return

	case StateGameOver:
		if rl.IsKeyPressed(rl.KeyR) {
			g.ResetGame()
			g.state = StatePlaying
		}
		if rl.IsKeyPressed(rl.KeyEscape) {
			g.state = StateMenu
		}
		return

	case StateUpgrade:
		if rl.IsKeyPressed(rl.KeyOne) {
			g.ApplyUpgrade(0)
		}
		if rl.IsKeyPressed(rl.KeyTwo) {
			g.ApplyUpgrade(1)
		}
		if rl.IsKeyPressed(rl.KeyThree) {
			g.ApplyUpgrade(2)
		}
		if rl.IsKeyPressed(rl.KeyFour) {
			g.ApplyUpgrade(3)
		}
		if rl.IsKeyPressed(rl.KeyFive) {
			g.ApplyUpgrade(4)
		}
		return

	case StatePaused:
		if rl.IsKeyPressed(rl.KeyP) {
			g.state = StatePlaying
		}
		if rl.IsKeyPressed(rl.KeyEscape) {
			g.state = StateMenu
		}
		return
	}

	// Playing state
	if rl.IsKeyPressed(rl.KeyP) {
		g.state = StatePaused
		return
	}

	g.gameTime += dt

	// Update players
	for pIdx := range g.players {
		player := &g.players[pIdx]

		// Update skill cooldowns
		for i := range player.skills {
			if !player.skills[i].ready {
				player.skills[i].cooldown -= dt
				if player.skills[i].cooldown <= 0 {
					player.skills[i].ready = true
					player.skills[i].cooldown = 0
				}
			}
		}

		// Player controls
		speed := player.stats.speed * dt
		newPos := player.position
		isMoving := false

		// Player 1: WASD
		if pIdx == 0 {
			if rl.IsKeyDown(rl.KeyW) {
				newPos.Z -= speed
				isMoving = true
			}
			if rl.IsKeyDown(rl.KeyS) {
				newPos.Z += speed
				isMoving = true
			}
			if rl.IsKeyDown(rl.KeyA) {
				newPos.X -= speed
				isMoving = true
				player.tiltAngle = float32(math.Min(float64(player.tiltAngle+dt*2), 0.1))
			} else if rl.IsKeyDown(rl.KeyD) {
				newPos.X += speed
				isMoving = true
				player.tiltAngle = float32(math.Max(float64(player.tiltAngle-dt*2), -0.1))
			} else {
				// Return tilt to neutral
				if player.tiltAngle > 0 {
					player.tiltAngle = float32(math.Max(0, float64(player.tiltAngle-dt*2)))
				} else {
					player.tiltAngle = float32(math.Min(0, float64(player.tiltAngle+dt*2)))
				}
			}

			// ยิงด้วยคลิกซ้ายหรือ Space
			if rl.IsMouseButtonDown(rl.MouseLeftButton) || rl.IsKeyDown(rl.KeySpace) {
				g.ShootBullet(player)
			}

			// คำนวณมุมหันจากตำแหน่งเมาส์
			mousePos := rl.GetMousePosition()
			screenPos := rl.GetWorldToScreen(player.position, g.camera)

			// คำนวณมุมระหว่างตำแหน่ง player กับเมาส์
			dx := mousePos.X - screenPos.X
			dy := mousePos.Y - screenPos.Y
			player.angle = float32(math.Atan2(float64(dy), float64(dx)))
		} else if pIdx == 1 && g.coopMode {
			// Player 2: Arrow Keys
			if rl.IsKeyDown(rl.KeyUp) {
				newPos.Z -= speed
				isMoving = true
			}
			if rl.IsKeyDown(rl.KeyDown) {
				newPos.Z += speed
				isMoving = true
			}
			if rl.IsKeyDown(rl.KeyLeft) {
				newPos.X -= speed
				isMoving = true
				player.tiltAngle = float32(math.Min(float64(player.tiltAngle+dt*2), 0.1))
			} else if rl.IsKeyDown(rl.KeyRight) {
				newPos.X += speed
				isMoving = true
				player.tiltAngle = float32(math.Max(float64(player.tiltAngle-dt*2), -0.1))
			} else {
				// Return tilt to neutral
				if player.tiltAngle > 0 {
					player.tiltAngle = float32(math.Max(0, float64(player.tiltAngle-dt*2)))
				} else {
					player.tiltAngle = float32(math.Min(0, float64(player.tiltAngle+dt*2)))
				}
			}

			// P2 shooting: NumPad 8/2/4/6 directional shoot, NumPad 0 = auto-aim nearest enemy
			if rl.IsKeyDown(rl.KeyKp8) {
				player.angle = -math.Pi / 2
				g.ShootBullet(player)
			}
			if rl.IsKeyDown(rl.KeyKp2) {
				player.angle = math.Pi / 2
				g.ShootBullet(player)
			}
			if rl.IsKeyDown(rl.KeyKp4) {
				player.angle = math.Pi
				g.ShootBullet(player)
			}
			if rl.IsKeyDown(rl.KeyKp6) {
				player.angle = 0
				g.ShootBullet(player)
			}
			// Auto-aim (NumPad 0) - ยิงไปยังศัตรูที่ใกล้สุดเมื่อกดครั้งเดียว
			if rl.IsKeyPressed(rl.KeyKp0) {
				var nearest *Enemy
				minD := float32(1e6)
				for i := range g.enemies {
					if !g.enemies[i].active {
						continue
					}
					dx := g.enemies[i].position.X - player.position.X
					dz := g.enemies[i].position.Z - player.position.Z
					d := float32(math.Sqrt(float64(dx*dx + dz*dz)))
					if d < minD {
						minD = d
						nearest = &g.enemies[i]
					}
				}
				if nearest != nil {
					ang := float32(math.Atan2(float64(nearest.position.Z-player.position.Z), float64(nearest.position.X-player.position.X)))
					player.angle = ang
					g.ShootBullet(player)
				}
			}
		}

		// --- Added: skill input handling (P1: Q/E/F, P2: Numpad 1/2/3 or 1/2/3) ---
		if pIdx == 0 {
			// Player 1 skills
			if rl.IsKeyPressed(rl.KeyQ) {
				g.UseSkill(player, 0)
			}
			if rl.IsKeyPressed(rl.KeyE) {
				g.UseSkill(player, 1)
			}
			if rl.IsKeyPressed(rl.KeyF) {
				g.UseSkill(player, 2)
			}
		} else if pIdx == 1 && g.coopMode {
			// Player 2 skills - try Numpad keys first, fallback to top-row numbers
			if rl.IsKeyPressed(rl.KeyKp1) || rl.IsKeyPressed(rl.KeyOne) {
				g.UseSkill(player, 0)
			}
			if rl.IsKeyPressed(rl.KeyKp2) || rl.IsKeyPressed(rl.KeyTwo) {
				g.UseSkill(player, 1)
			}
			if rl.IsKeyPressed(rl.KeyKp3) || rl.IsKeyPressed(rl.KeyThree) {
				g.UseSkill(player, 2)
			}
		}

		// Apply movement: check collision then commit new position
		// (prevent walking through obstacles)
		if !g.CheckObstacleCollision(newPos, 0.9) {
			player.position = newPos
		} else {
			// ถ้าชน obstacle อยู่ ให้ไม่ย้ายตำแหน่ง (สามารถปรับเป็น slide ได้ถ้าต้องการ)
		}

		// Ensure player stays inside map/stage bounds
		g.clampPlayerToStageBounds(player, 0.9)

		// Apply movement state to player for animations
		player.isMoving = isMoving
	}

	// Update bullets
	for i := range g.bullets {
		if g.bullets[i].active {
			newPos := rl.Vector3{
				X: g.bullets[i].position.X + g.bullets[i].velocity.X*dt,
				Y: g.bullets[i].position.Y,
				Z: g.bullets[i].position.Z + g.bullets[i].velocity.Z*dt,
			}

			// Check obstacle collision
			if g.CheckObstacleCollision(newPos, 0.3) {
				g.bullets[i].active = false
				g.CreateExplosion(g.bullets[i].position, rl.Yellow, 5)
				continue
			}

			g.bullets[i].position = newPos

			if math.Abs(float64(g.bullets[i].position.Z)) > 40 ||
				math.Abs(float64(g.bullets[i].position.X)) > 40 {
				g.bullets[i].active = false
			}
		}
	}

	// Boss spawn check
	if g.level%5 == 0 && !g.bossSpawned {
		g.SpawnBoss()
	}

	// Spawn enemies
	if !g.bossActive {
		g.spawnTimer += dt
		if g.spawnTimer > g.spawnInterval {
			g.spawnTimer = 0
			activeCount := 0
			for i := range g.enemies {
				if g.enemies[i].active && !g.enemies[i].isBoss {
					activeCount++
				}
			}
			if activeCount < 10+g.level*2 {
				g.SpawnEnemy()
			}
		}
	}

	// Update enemies
	for i := range g.enemies {
		if !g.enemies[i].active {
			continue
		}

		// Find nearest player
		nearestPlayer := &g.players[0]
		minDist := float32(999999.0)

		for pIdx := range g.players {
			dx := g.players[pIdx].position.X - g.enemies[i].position.X
			dz := g.players[pIdx].position.Z - g.enemies[i].position.Z
			dist := float32(math.Sqrt(float64(dx*dx + dz*dz)))

			if dist < minDist {
				minDist = dist
				nearestPlayer = &g.players[pIdx]
			}
		}

		if g.enemies[i].isBoss {
			// Boss: เคลื่อนที่ตรงไปหาผู้เล่น + วนรอบเล็กน้อย
			dx := nearestPlayer.position.X - g.enemies[i].position.X
			dz := nearestPlayer.position.Z - g.enemies[i].position.Z
			dist := float32(math.Sqrt(float64(dx*dx + dz*dz)))

			if dist > 5.0 { // ถ้าไกล ก็เข้าใกล้
				speed := float32(6.0 + float64(g.level)*0.3)
				newPos := rl.Vector3{
					X: g.enemies[i].position.X + (dx/dist)*speed*dt,
					Y: g.enemies[i].position.Y,
					Z: g.enemies[i].position.Z + (dz/dist)*speed*dt,
				}

				if !g.CheckObstacleCollision(newPos, g.enemies[i].size/2) {
					g.enemies[i].position = newPos
				}
			} else { // ถ้าใกล้แล้ว ก็วนรอบ
				angle := g.gameTime * 1.0
				radius := float32(8.0)
				targetX := nearestPlayer.position.X + float32(math.Cos(float64(angle)))*radius
				targetZ := nearestPlayer.position.Z + float32(math.Sin(float64(angle)))*radius

				dx = targetX - g.enemies[i].position.X
				dz = targetZ - g.enemies[i].position.Z
				speed := float32(8.0)

				newPos := rl.Vector3{
					X: g.enemies[i].position.X + dx*speed*dt*0.1,
					Y: g.enemies[i].position.Y,
					Z: g.enemies[i].position.Z + dz*speed*dt*0.1,
				}

				if !g.CheckObstacleCollision(newPos, g.enemies[i].size/2) {
					g.enemies[i].position = newPos
				}
			}
		} else {
			// Normal enemy: ไล่ตามผู้เล่น
			dx := nearestPlayer.position.X - g.enemies[i].position.X
			dz := nearestPlayer.position.Z - g.enemies[i].position.Z
			dist := float32(math.Sqrt(float64(dx*dx + dz*dz)))

			if dist > 0.1 {
				speed := float32(math.Sqrt(float64(g.enemies[i].velocity.X*g.enemies[i].velocity.X +
					g.enemies[i].velocity.Z*g.enemies[i].velocity.Z)))
				g.enemies[i].velocity.X = dx / dist * speed
				g.enemies[i].velocity.Z = dz / dist * speed
			}

			newPos := rl.Vector3{
				X: g.enemies[i].position.X + g.enemies[i].velocity.X*dt,
				Y: g.enemies[i].position.Y,
				Z: g.enemies[i].position.Z + g.enemies[i].velocity.Z*dt,
			}

			if !g.CheckObstacleCollision(newPos, g.enemies[i].size/2) {
				g.enemies[i].position = newPos
			}
		}

		// Collision with players
		for pIdx := range g.players {
			player := &g.players[pIdx]
			dx := player.position.X - g.enemies[i].position.X
			dz := player.position.Z - g.enemies[i].position.Z
			playerDist := float32(math.Sqrt(float64(dx*dx + dz*dz)))

			collisionDist := float32(1.5)
			if g.enemies[i].isBoss {
				collisionDist = 3.0
			}

			if playerDist < collisionDist {
				damage := 20
				if g.enemies[i].isBoss {
					damage = 30
				}
				player.health -= damage
				g.CreateExplosion(player.position, rl.Red, 10)
				g.playSound(g.sounds.hit)

				if playerDist > 0 {
					pushDist := float32(3.0)
					g.enemies[i].position.X += (g.enemies[i].position.X - player.position.X) / playerDist * pushDist
					g.enemies[i].position.Z += (g.enemies[i].position.Z - player.position.Z) / playerDist * pushDist
				}

				if player.health <= 0 {
					g.state = StateGameOver
					if g.score > g.highScore {
						g.highScore = g.score
					}
				}
			}
		}

		// Collision with bullets
		for j := range g.bullets {
			if g.bullets[j].active {
				dx := g.bullets[j].position.X - g.enemies[i].position.X
				dz := g.bullets[j].position.Z - g.enemies[i].position.Z
				dist := math.Sqrt(float64(dx*dx + dz*dz))

				if dist < float64(g.enemies[i].size) {
					g.enemies[i].health -= g.bullets[j].damage
					g.bullets[j].active = false
					g.CreateExplosion(g.enemies[i].position, rl.Yellow, 5)

					if g.enemies[i].health <= 0 {
						g.KillEnemy(i)
					}
				}
			}
		}
	}

	// Update particles
	for i := range g.particles {
		if g.particles[i].active {
			g.particles[i].position.X += g.particles[i].velocity.X * dt
			g.particles[i].position.Y += g.particles[i].velocity.Y * dt
			g.particles[i].position.Z += g.particles[i].velocity.Z * dt
			g.particles[i].velocity.Y -= 20 * dt
			g.particles[i].lifetime -= dt

			if g.particles[i].lifetime <= 0 {
				g.particles[i].active = false
			}
		}
	}

	// Update power-ups
	for i := range g.powerUps {
		if g.powerUps[i].active {
			g.powerUps[i].rotation += 90 * dt

			for pIdx := range g.players {
				player := &g.players[pIdx]
				dx := player.position.X - g.powerUps[i].position.X
				dz := player.position.Z - g.powerUps[i].position.Z
				dist := math.Sqrt(float64(dx*dx + dz*dz))

				if dist < 2.0 {
					g.powerUps[i].active = false

					switch g.powerUps[i].pType {
					case 0:
						player.health = int(math.Min(float64(player.health+30), float64(player.stats.maxHealth)))
					case 1:
						player.stats.speed = float32(math.Min(float64(player.stats.speed+2), 20))
					case 2:
						player.stats.fireRate = float32(math.Max(float64(player.stats.fireRate-0.02), 0.05))
					}

					g.CreateExplosion(g.powerUps[i].position, rl.Green, 8)
					g.playSound(g.sounds.powerup)
					break
				}
			}
		}
	}

	// Update camera
	var centerX, centerZ float32
	for _, player := range g.players {
		centerX += player.position.X
		centerZ += player.position.Z
	}
	centerX /= float32(len(g.players))
	centerZ /= float32(len(g.players))

	distance := float32(30.0)
	g.camera.Position = rl.NewVector3(
		centerX+distance*0.707,
		distance*0.707,
		centerZ+distance*0.707,
	)
	g.camera.Target = rl.NewVector3(centerX, 0, centerZ)
}

func (g *Game) DrawMenu() {
	rl.ClearBackground(rl.NewColor(10, 10, 25, 255))

	centerX := int32(screenWidth / 2)

	rl.DrawText("3D SHOOTER", centerX-200, 100, 70, rl.Gold)
	rl.DrawText("CO-OP EDITION", centerX-180, 180, 35, rl.Yellow)

	menuItems := []string{
		"Single Player",
		"Co-op Mode",
		"Settings",
		"Quit",
	}

	for i, item := range menuItems {
		y := int32(300 + i*60)
		color := rl.White

		if i == g.menuSelection {
			color = rl.Yellow
			rl.DrawRectangle(centerX-200, y-5, 400, 50, rl.NewColor(255, 255, 0, 50))
			rl.DrawText(">", centerX-250, y, 40, rl.Yellow)
		}

		rl.DrawText(item, centerX-150, y, 40, color)
	}

	rl.DrawText("Use UP/DOWN arrows and ENTER to select", centerX-250, screenHeight-80, 20, rl.LightGray)

	if g.highScore > 0 {
		rl.DrawText(fmt.Sprintf("High Score: %d", g.highScore), centerX-100, screenHeight-40, 25, rl.Gold)
	}
}

func (g *Game) DrawSettings() {
	rl.ClearBackground(rl.NewColor(10, 10, 25, 255))

	centerX := int32(screenWidth / 2)

	rl.DrawText("SETTINGS", centerX-120, 80, 50, rl.Gold)

	settingsY := int32(200)

	settings := []struct {
		name  string
		value string
	}{
		{"Sound Effects", func() string {
			if g.settings.soundEnabled {
				return "ON"
			}
			return "OFF"
		}()},
		{"Music", func() string {
			if g.settings.musicEnabled {
				return "ON"
			}
			return "OFF"
		}()},
		{"Sound Volume", fmt.Sprintf("%.0f%%", g.settings.soundVolume*100)},
		{"Music Volume", fmt.Sprintf("%.0f%%", g.settings.musicVolume*100)},
		{"Difficulty", func() string {
			switch g.settings.difficulty {
			case 0:
				return "EASY"
			case 1:
				return "NORMAL"
			case 2:
				return "HARD"
			default:
				return "NORMAL"
			}
		}()}, // Fixed: Added missing parentheses and comma
		{"Back", ""},
	}

	for i, setting := range settings {
		y := settingsY + int32(i*60)
		color := rl.White

		if i == g.settingsSelection {
			color = rl.Yellow
			rl.DrawRectangle(centerX-300, y-5, 600, 50, rl.NewColor(255, 255, 0, 50))
			rl.DrawText(">", centerX-350, y, 35, rl.Yellow)
		}

		rl.DrawText(setting.name, centerX-280, y, 35, color)

		if setting.value != "" {
			valueColor := color
			if i < 4 {
				valueColor = rl.Lime
			}
			rl.DrawText(setting.value, centerX+100, y, 35, valueColor)
		}
	}

	rl.DrawText("Use UP/DOWN to navigate, LEFT/RIGHT to change", centerX-280, screenHeight-80, 20, rl.LightGray)
	rl.DrawText("Press ESC or select Back to return", centerX-200, screenHeight-50, 20, rl.LightGray)
}

func (g *Game) DrawGame() {
	rl.BeginMode3D(g.camera)

	// Draw floor with stage-specific color
	floorColor := rl.NewColor(30, 30, 50, 255)
	switch g.currentStage {
	case StageMaze:
		floorColor = rl.NewColor(40, 30, 50, 255)
	case StageHazard:
		floorColor = rl.NewColor(50, 30, 30, 255)
	case StageArena:
		floorColor = rl.NewColor(30, 40, 50, 255)
	}
	rl.DrawPlane(rl.NewVector3(0, 0, 0), rl.NewVector2(60, 60), floorColor)

	// Grid
	gridSize := 60
	gridStep := 3
	for i := -gridSize / 2; i <= gridSize/2; i += gridStep {
		rl.DrawLine3D(
			rl.NewVector3(float32(i), 0, float32(-gridSize/2)),
			rl.NewVector3(float32(i), 0, float32(gridSize/2)),
			rl.NewColor(40, 40, 60, 255),
		)
		rl.DrawLine3D(
			rl.NewVector3(float32(-gridSize/2), 0, float32(i)),
			rl.NewVector3(float32(gridSize/2), 0, float32(i)),
			rl.NewColor(40, 40, 60, 255),
		)
	}

	// Draw obstacles
	for i := range g.obstacles {
		if g.obstacles[i].active {
			if g.obstacles[i].obsType == 0 {
				// Wall
				rl.DrawCube(g.obstacles[i].position, g.obstacles[i].size.X, g.obstacles[i].size.Y, g.obstacles[i].size.Z, rl.NewColor(100, 100, 120, 255))
				rl.DrawCubeWires(g.obstacles[i].position, g.obstacles[i].size.X, g.obstacles[i].size.Y, g.obstacles[i].size.Z, rl.White)
			} else {
				// Hazard
				rl.DrawCube(g.obstacles[i].position, g.obstacles[i].size.X, g.obstacles[i].size.Y, g.obstacles[i].size.Z, rl.NewColor(200, 50, 50, 150))
				rl.DrawCubeWires(g.obstacles[i].position, g.obstacles[i].size.X, g.obstacles[i].size.Y, g.obstacles[i].size.Z, rl.Red)
			}
		}
	}

	// Draw players
	for _, player := range g.players {
		playerColor := player.color
		if player.health < 30 {
			playerColor = rl.Orange
		}

		if g.modelsLoaded && player.model.MeshCount > 0 {
			// คำนวณมุมหมุนจากทิศทางที่ชี้
			angleDeg := player.angle*180.0/math.Pi + player.modelYawOffsetDeg

			position := player.position
			position.Y += 0.5 // ยกโมเดลขึ้นเล็กน้อย

			rl.DrawModelEx(
				player.model,
				position,
				rl.NewVector3(0, 1, 0), // แกนหมุน Y
				angleDeg,               // มุมหมุน
				rl.NewVector3(player.modelScale, player.modelScale, player.modelScale),
				playerColor)
		} else {
			// Fallback to cube if no model
			rl.DrawCube(player.position, 1.2, 1.8, 1.2, playerColor)
			rl.DrawCubeWires(player.position, 1.2, 1.8, 1.2, rl.White)
		}

		// Direction line
		dirLen := float32(2.0)
		dirEnd := rl.NewVector3(
			player.position.X+float32(math.Cos(float64(player.angle)))*dirLen,
			player.position.Y,
			player.position.Z+float32(math.Sin(float64(player.angle)))*dirLen,
		)
		rl.DrawLine3D(player.position, dirEnd, rl.Yellow)
		rl.DrawSphere(dirEnd, 0.2, rl.Yellow)
	}

	// Draw bullets
	for i := range g.bullets {
		if g.bullets[i].active {
			bulletColor := rl.Yellow
			if g.bullets[i].playerId == 1 {
				bulletColor = rl.Lime
			}
			rl.DrawSphere(g.bullets[i].position, 0.3, bulletColor)
		}
	}

	// Draw enemies
	for i := range g.enemies {
		if g.enemies[i].active {
			if g.enemies[i].hasModel && g.enemies[i].model.MeshCount > 0 {
				scale := g.enemies[i].modelScale
				// Boss uses boss model assigned in SpawnBoss; others use enemyModel
				if g.enemies[i].isBoss {
					rl.DrawModelEx(g.bossModel, g.enemies[i].position, rl.NewVector3(0, 1, 0), g.enemies[i].modelYawOffsetDeg, rl.NewVector3(scale, scale, scale), g.enemies[i].color)
				} else {
					rl.DrawModelEx(g.enemyModel, g.enemies[i].position, rl.NewVector3(0, 1, 0), g.enemies[i].modelYawOffsetDeg, rl.NewVector3(scale, scale, scale), g.enemies[i].color)
				}
			} else {
				rl.DrawCube(g.enemies[i].position, g.enemies[i].size, g.enemies[i].size, g.enemies[i].size, g.enemies[i].color)
				rl.DrawCubeWires(g.enemies[i].position, g.enemies[i].size, g.enemies[i].size, g.enemies[i].size, rl.Maroon)
			}

			// Boss HP bar
			if g.enemies[i].isBoss {
				healthPercent := float32(g.enemies[i].health) / float32(g.enemies[i].maxHealth)
				barWidth := float32(5.0)
				barHeight := float32(0.5)
				barPos := g.enemies[i].position
				barPos.Y += g.enemies[i].size + 1

				rl.DrawCube(barPos, barWidth, barHeight, 0.1, rl.DarkGray)
				healthBarPos := barPos
				healthBarPos.X -= barWidth/2 - (barWidth*healthPercent)/2
				rl.DrawCube(healthBarPos, barWidth*healthPercent, barHeight, 0.1, rl.Red)
			}
		}
	}

	// Draw particles
	for i := range g.particles {
		if g.particles[i].active {
			size := g.particles[i].lifetime * 0.4
			rl.DrawSphere(g.particles[i].position, size, g.particles[i].color)
		}
	}

	// Draw power-ups
	for i := range g.powerUps {
		if g.powerUps[i].active {
			pos := g.powerUps[i].position
			pos.Y += float32(math.Sin(float64(g.gameTime*3))) * 0.3

			var color rl.Color
			switch g.powerUps[i].pType {
			case 0:
				color = rl.Green
			case 1:
				color = rl.SkyBlue
			case 2:
				color = rl.Magenta
			}

			rl.DrawCube(pos, 0.8, 0.8, 0.8, color)
			rl.DrawCubeWires(pos, 0.8, 0.8, 0.8, rl.White)
		}
	}

	rl.EndMode3D()

	// UI
	rl.DrawRectangle(10, 10, 450, 180, rl.NewColor(0, 0, 0, 150))
	rl.DrawText(fmt.Sprintf("Score: %d", g.score), 20, 20, 25, rl.White)
	rl.DrawText(fmt.Sprintf("Level: %d", g.level), 20, 50, 20, rl.Yellow)

	// Stage indicator
	stageNames := []string{"BASIC", "MAZE", "HAZARD", "ARENA"}
	stageName := stageNames[int(g.currentStage)]
	rl.DrawText(fmt.Sprintf("Stage: %s", stageName), 20, 75, 18, rl.NewColor(0, 255, 255, 255))

	if g.bossActive {
		rl.DrawText("WARNING: BOSS FIGHT!", 20, 100, 22, rl.Red)
	} else {
		rl.DrawText(fmt.Sprintf("Enemies: %d", g.enemiesKilled), 20, 100, 18, rl.LightGray)
	}

	// Player stats
	rl.DrawText(fmt.Sprintf("DMG: %d | SPD: %.0f | CRIT: %.0f%%",
		g.players[0].stats.damage, g.players[0].stats.speed, g.players[0].stats.critChance*100), 20, 125, 16, rl.Lime)

	// Health bars
	healthBarY := int32(150)
	for pIdx, player := range g.players {
		healthPercent := float32(player.health) / float32(player.stats.maxHealth)
		healthColor := rl.Green
		if healthPercent < 0.3 {
			healthColor = rl.Red
		} else if healthPercent < 0.6 {
			healthColor = rl.Orange
		}

		yPos := healthBarY + int32(pIdx*35)

		playerLabel := fmt.Sprintf("P%d", pIdx+1)
		rl.DrawText(playerLabel, 20, yPos, 18, player.color)

		rl.DrawRectangle(50, yPos, 380, 25, rl.DarkGray)
		rl.DrawRectangle(50, yPos, int32(380*healthPercent), 25, healthColor)
		rl.DrawText(fmt.Sprintf("HP: %d/%d", player.health, player.stats.maxHealth), 55, yPos+3, 16, rl.White)
	}

	// Skills UI
	skillY := int32(230)
	if g.coopMode {
		skillY = 265
	}
	rl.DrawRectangle(10, skillY, 450, 140, rl.NewColor(0, 0, 0, 150))
	rl.DrawText("=== P1 SKILLS ===", 20, skillY+10, 20, rl.Lime)

	skillKeys := []string{"Q", "E", "F"}
	for i := range g.players[0].skills {
		y := skillY + 40 + int32(i*30)
		keyText := fmt.Sprintf("[%s]", skillKeys[i])
		skillName := g.players[0].skills[i].name

		if g.players[0].skills[i].ready {
			rl.DrawText(fmt.Sprintf("%s %s [READY]", keyText, skillName), 20, y, 18, rl.Green)
		} else {
			cooldownLeft := g.players[0].skills[i].cooldown
			rl.DrawText(fmt.Sprintf("%s %s [%.1fs]", keyText, skillName, cooldownLeft), 20, y, 18, rl.Gray)

			cdPercent := 1.0 - (cooldownLeft / g.players[0].skills[i].maxCooldown)
			rl.DrawRectangle(240, y, 200, 15, rl.DarkGray)
			rl.DrawRectangle(240, y, int32(200*cdPercent), 15, rl.Yellow)
		}
	}

	// P2 Skills
	if g.coopMode {
		skillY2 := int32(420)
		rl.DrawRectangle(10, skillY2, 450, 180, rl.NewColor(0, 0, 0, 150))
		rl.DrawText("=== P2 SKILLS ===", 20, skillY2+10, 20, rl.Lime)

		skillKeys2 := []string{"Num1", "Num2", "Num3"}
		for i := range g.players[1].skills {
			y := skillY2 + 40 + int32(i*30)
			keyText := fmt.Sprintf("[%s]", skillKeys2[i])
			skillName := g.players[1].skills[i].name

			if g.players[1].skills[i].ready {
				rl.DrawText(fmt.Sprintf("%s %s [READY]", keyText, skillName), 20, y, 18, rl.Green)
			} else {
				cooldownLeft := g.players[1].skills[i].cooldown
				rl.DrawText(fmt.Sprintf("%s %s [%.1fs]", keyText, skillName, cooldownLeft), 20, y, 18, rl.Gray)

				cdPercent := 1.0 - (cooldownLeft / g.players[1].skills[i].maxCooldown)
				rl.DrawRectangle(240, y, 200, 15, rl.DarkGray)
				rl.DrawRectangle(240, y, int32(200*cdPercent), 15, rl.Yellow)
			}
		}

		// P2 Shooting controls
		rl.DrawText("NumPad 2468: Shoot | 0: Auto-aim", 20, skillY2+130, 14, rl.LightGray)
	}

	// Controls
	if g.coopMode {
		rl.DrawText("P1: WASD+QEF+Mouse | P2: Arrows+NumPad(2468=Shoot,123=Skills,0=Auto) | P: Pause", 10, screenHeight-30, 12, rl.LightGray)
	} else {
		rl.DrawText("WASD: Move | Mouse/Space: Shoot | Q/E/F: Skills | P: Pause", 10, screenHeight-30, 14, rl.LightGray)
	}

	// Boss warning
	if g.level%5 == 0 && !g.bossSpawned && g.level > 0 {
		flashTime := int(g.gameTime * 3)
		if flashTime%2 == 0 {
			rl.DrawText("!!! BOSS INCOMING !!!", screenWidth/2-180, 100, 40, rl.Red)
		}
	}

	// Stage change warning
	nextStageLevel := ((g.level / stageInterval) + 1) * stageInterval
	if nextStageLevel-g.level <= 2 && nextStageLevel-g.level > 0 {
		rl.DrawText(fmt.Sprintf("New Stage in %d levels!", nextStageLevel-g.level),
			screenWidth/2-150, 150, 25, rl.Orange)
	}

	// FPS
	rl.DrawText(fmt.Sprintf("FPS: %d", rl.GetFPS()), screenWidth-100, 10, 20, rl.Green)
}

func (g *Game) DrawUpgrade() {
	rl.DrawRectangle(0, 0, screenWidth, screenHeight, rl.NewColor(0, 0, 0, 200))

	centerX := int32(screenWidth / 2)
	centerY := int32(screenHeight / 2)

	rl.DrawText("LEVEL UP!", centerX-150, centerY-200, 50, rl.Gold)
	rl.DrawText("Choose an Upgrade:", centerX-180, centerY-140, 30, rl.White)

	upgrades := []string{
		"[1] Max Health +20",
		"[2] Damage +1",
		"[3] Speed +2",
		"[4] Fire Rate +10%",
		"[5] Crit Chance +5%",
	}

	for i, upgrade := range upgrades {
		y := centerY - 80 + int32(i)*50
		color := rl.White

		if i == g.upgradeChoice {
			color = rl.Yellow
			rl.DrawRectangle(centerX-250, y-5, 500, 40, rl.NewColor(255, 255, 0, 50))
		}

		rl.DrawText(upgrade, centerX-240, y, 25, color)
	}

	rl.DrawText("Press 1-5 to choose", centerX-150, centerY+150, 20, rl.LightGray)

	// Current stats
	statsY := int32(50)
	rl.DrawRectangle(screenWidth-320, statsY, 310, 180, rl.NewColor(0, 0, 0, 150))
	rl.DrawText("Current Stats:", screenWidth-310, statsY+10, 20, rl.Lime)
	rl.DrawText(fmt.Sprintf("Max HP: %d", g.players[0].stats.maxHealth), screenWidth-310, statsY+40, 18, rl.White)
	rl.DrawText(fmt.Sprintf("Damage: %d", g.players[0].stats.damage), screenWidth-310, statsY+65, 18, rl.White)
	rl.DrawText(fmt.Sprintf("Speed: %.1f", g.players[0].stats.speed), screenWidth-310, statsY+90, 18, rl.White)
	rl.DrawText(fmt.Sprintf("Fire Rate: %.2fs", g.players[0].stats.fireRate), screenWidth-310, statsY+115, 18, rl.White)
	rl.DrawText(fmt.Sprintf("Crit: %.0f%%", g.players[0].stats.critChance*100), screenWidth-310, statsY+140, 18, rl.White)
}

func (g *Game) DrawPaused() {
	rl.DrawRectangle(0, 0, screenWidth, screenHeight, rl.NewColor(0, 0, 0, 150))
	rl.DrawText("PAUSED", screenWidth/2-100, screenHeight/2-30, 40, rl.White)
	rl.DrawText("Press P to Resume", screenWidth/2-120, screenHeight/2+20, 25, rl.Green)
	rl.DrawText("Press ESC for Menu", screenWidth/2-120, screenHeight/2+55, 25, rl.Yellow)
}

func (g *Game) DrawGameOver() {
	rl.DrawRectangle(0, 0, screenWidth, screenHeight, rl.NewColor(0, 0, 0, 180))
	rl.DrawText("GAME OVER!", screenWidth/2-180, screenHeight/2-100, 60, rl.Red)
	rl.DrawText(fmt.Sprintf("Final Score: %d", g.score), screenWidth/2-150, screenHeight/2-20, 35, rl.White)

	rl.DrawText(fmt.Sprintf("Max Level: %d", g.level), screenWidth/2-120, screenHeight/2+25, 28, rl.Yellow)
	rl.DrawText(fmt.Sprintf("Enemies Killed: %d", g.enemiesKilled), screenWidth/2-140, screenHeight/2+60, 25, rl.LightGray)
	if g.highScore > 0 {
		rl.DrawText(fmt.Sprintf("High Score: %d", g.highScore), screenWidth/2-130, screenHeight/2+95, 25, rl.Gold)
	}
	rl.DrawText("Press R to Restart", screenWidth/2-130, screenHeight/2+135, 28, rl.Green)
	rl.DrawText("Press ESC for Menu", screenWidth/2-130, screenHeight/2+170, 28, rl.Yellow)
}

func (g *Game) Draw() {

	rl.BeginDrawing()

	switch g.state {
	case StateMenu:
		g.DrawMenu()
	case StateSettings:
		g.DrawSettings()
	case StatePlaying:
		rl.ClearBackground(rl.NewColor(10, 10, 25, 255))
		g.DrawGame()
	case StatePaused:
		rl.ClearBackground(rl.NewColor(10, 10, 25, 255))
		g.DrawGame()
		g.DrawPaused()
	case StateUpgrade:
		rl.ClearBackground(rl.NewColor(10, 10, 25, 255))
		g.DrawGame()
		g.DrawUpgrade()
	case StateGameOver:
		rl.ClearBackground(rl.NewColor(10, 10, 25, 255))
		g.DrawGame()
		g.DrawGameOver()
	}

	rl.EndDrawing()
}

func main() {
	rand.Seed(time.Now().UnixNano())

	rl.InitWindow(screenWidth, screenHeight, "3D Co-op Shooter - Enhanced")
	defer rl.CloseWindow()

	rl.SetTargetFPS(60)

	game := NewGame()
	defer rl.CloseAudioDevice()

	for !rl.WindowShouldClose() {
		dt := rl.GetFrameTime()
		game.Update(dt)
		game.Draw()

	}
}
