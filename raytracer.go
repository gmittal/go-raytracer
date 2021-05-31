package main

import (
	"math"
	"sync"

	"github.com/fogleman/gg"
)

type Canvas struct {
	lock sync.Mutex
	wg   sync.WaitGroup
	ctx  *gg.Context
}

type Color struct {
	r float64
	g float64
	b float64
}

type Vector struct {
	x float64
	y float64
	z float64
}

type Sphere struct {
	center     Vector
	radius     float64
	color      Color
	specular   float64 // shininess
	reflective float64
}

type Light struct {
	kind      string // TODO: change to enum
	intensity float64
	position  Vector
	direction Vector
}

func (c *Canvas) PutPixel(x int, y int, color Color) {
	defer c.wg.Done()
	i, j := ChangeCoord2D(x, y)
	c.lock.Lock()
	c.ctx.SetPixel(i, j)
	c.ctx.SetRGB(color.r, color.g, color.b)
	c.ctx.Fill()
	c.lock.Unlock()
}

func MakeVector(x float64, y float64, z float64) Vector {
	var p Vector
	p.x = x
	p.y = y
	p.z = z
	return p
}

func MakeColor(r float64, g float64, b float64) Color {
	var c Color
	c.r = math.Max(math.Min(r, 1.0), 0.0)
	c.g = math.Max(math.Min(g, 1.0), 0.0)
	c.b = math.Max(math.Min(b, 1.0), 0.0)
	return c
}

func WeightColor(c Color, w float64) Color {
	return MakeColor(c.r*w, c.g*w, c.b*w)
}

func AddColors(c1 Color, c2 Color) Color {
	return MakeColor(c1.r+c2.r, c1.g+c2.g, c1.b+c2.b)
}

func MakeSphere(center Vector, radius float64, color Color, specular float64, reflective float64) Sphere {
	var s Sphere
	s.center = center
	s.radius = radius
	s.color = color
	s.specular = specular
	s.reflective = reflective
	return s
}

func MakeLight(kind string, intensity float64, position Vector, direction Vector) Light {
	var l Light
	l.kind = kind
	l.intensity = intensity
	l.position = position
	l.direction = direction
	return l
}

func ChangeCoord2D(cx int, cy int) (int, int) {
	// Change coords from [-C/2, C/2] to [0, C]
	return Cw/2 + cx, Ch/2 - cy
}

func dot(a Vector, b Vector) float64 {
	return a.x*b.x + a.y*b.y + a.z*b.z
}

func sub(a Vector, b Vector) Vector {
	return MakeVector(a.x-b.x, a.y-b.y, a.z-b.z)
}

func add(a Vector, b Vector) Vector {
	return MakeVector(a.x+b.x, a.y+b.y, a.z+b.z)
}

func mul(a Vector, b Vector) Vector {
	return MakeVector(a.x*b.x, a.y*b.y, a.z*b.z)
}

func neg(a Vector) Vector {
	return MakeVector(-a.x, -a.y, -a.z)
}

func norm(a Vector) float64 {
	return math.Sqrt(dot(a, a))
}

func normalize(a Vector) Vector {
	length := norm(a)
	return MakeVector(a.x/length, a.y/length, a.z/length)
}

const Vw, Vh = 1, 1
const Cw, Ch = 1024, 1024
const d = 1

func main() {
	O := MakeVector(0, 0, -3)
	var canvas Canvas
	canvas.ctx = gg.NewContext(Cw, Ch)

	// Define scene.
	s1 := MakeSphere(MakeVector(0, -1, 3), 1, MakeColor(1.0, 0, 0), 500, 0.2)
	s2 := MakeSphere(MakeVector(2, 0, 4), 1, MakeColor(0., 0., 1.0), 500, 0.3)
	s3 := MakeSphere(MakeVector(-2, 0, 4), 1, MakeColor(0., 1.0, 0.), 10, 0.4)
	s4 := MakeSphere(MakeVector(0, -5001, 0), 5000, MakeColor(1.0, 1.0, 0), 1000, 0.5)
	spheres := []*Sphere{&s1, &s2, &s3, &s4}

	l1 := MakeLight("ambient", 0.2, MakeVector(0, 0, 0), MakeVector(0, 0, 0))
	l2 := MakeLight("point", 0.6, MakeVector(2, 1, 0), MakeVector(0, 0, 0))
	l3 := MakeLight("directional", 0.2, MakeVector(0, 0, 0), MakeVector(1, 4, 4))
	lights := []*Light{&l1, &l2, &l3}

	max_recursion_depth := 3 // for recursive raytracing of reflections

	// Draw scene.
	for x := -Cw / 2; x < Cw/2; x++ {
		for y := -Ch / 2; y < Ch/2; y++ {
			D := CanvasToViewPort(x, y) // TODO: Add support for camera rotation (left-multiply by rotation matrix)
			canvas.wg.Add(1)
			func(spheres []*Sphere, lights []*Light, O Vector, D Vector, t_min float64, t_max float64, r int, x int, y int) {
				color := TraceRay(spheres, lights, O, D, t_min, t_max, r)
				canvas.PutPixel(x, y, color)
			}(spheres, lights, O, D, 1, math.Inf(1), max_recursion_depth, x, y)
		}
	}

	canvas.wg.Wait()
	canvas.ctx.SavePNG("out.png")
}

func TraceRay(spheres []*Sphere, lights []*Light, origin Vector, direction Vector, t_min float64, t_max float64, recursion_depth int) Color {
	best_sphere, best_t := ClosestIntersection(spheres, origin, direction, t_min, t_max)

	if best_sphere == nil {
		return MakeColor(0.0, 0.0, 0.0) // default background color
	}

	// Lighting
	t := MakeVector(best_t, best_t, best_t)
	intersection_pt := add(origin, mul(t, direction))
	normal := normalize(sub(intersection_pt, best_sphere.center))
	intensity := Lighting(spheres, lights, intersection_pt, normal, neg(direction), best_sphere.specular)
	local_color := WeightColor(best_sphere.color, intensity)

	// Reflections
	r := best_sphere.reflective
	if recursion_depth <= 0 || r <= 0 {
		return local_color
	}
	R := ReflectRay(neg(direction), normal)
	reflected_color := TraceRay(spheres, lights, intersection_pt, R, 0.001, math.Inf(1), recursion_depth-1)

	return AddColors(WeightColor(local_color, (1-r)), WeightColor(reflected_color, r))
}

func ClosestIntersection(spheres []*Sphere, origin Vector, direction Vector, t_min float64, t_max float64) (*Sphere, float64) {
	best_t := t_max
	var best_sphere *Sphere
	best_sphere = nil

	for _, sphere := range spheres {
		t1, t2 := IntersectRaySphere(origin, direction, *sphere)
		if t1 < best_t && t_min <= t1 && t1 <= t_max {
			best_sphere = sphere
			best_t = t1
		}
		if t2 < best_t && t_min <= t2 && t2 <= t_max {
			best_sphere = sphere
			best_t = t2
		}
	}
	return best_sphere, best_t
}

func IntersectRaySphere(origin Vector, direction Vector, sphere Sphere) (float64, float64) {
	// NOTE: This math only works for spheres...
	// TODO: make this adaptable for any arbitrary object
	r := sphere.radius
	CO := sub(origin, sphere.center)

	// Solve quadratic
	// TODO: Add caching
	a := dot(direction, direction)
	b := 2 * dot(CO, direction)
	c := dot(CO, CO) - r*r
	discrim := b*b - 4*a*c
	if discrim < 0 {
		return math.Inf(1), math.Inf(1)
	}
	t1 := (-b + math.Sqrt(discrim)) / (2 * a)
	t2 := (-b - math.Sqrt(discrim)) / (2 * a)
	return t1, t2
}

func ReflectRay(ray Vector, normal Vector) Vector {
	k := 2 * dot(normal, ray)
	return sub(mul(MakeVector(k, k, k), normal), ray)
}

func Lighting(spheres []*Sphere, lights []*Light, point Vector, normal Vector, reflection Vector, specular float64) float64 {
	intensity := 0.
	for _, light := range lights {
		if light.kind == "ambient" {
			intensity += light.intensity
		} else {
			var L Vector
			var t_max float64
			if light.kind == "point" {
				L = sub(light.position, point)
				t_max = 1
			} else { // directional
				L = light.direction
				t_max = math.Inf(1)
			}

			// Shadows
			shadow_sphere, _ := ClosestIntersection(spheres, point, L, 0.001, t_max)
			if shadow_sphere != nil {
				continue
			}

			// Diffusion
			N := normalize(normal)
			L = normalize(L)
			intensity += light.intensity * math.Max(0, dot(N, L))

			// Specular
			if specular != -1 {
				R := normalize(ReflectRay(L, N))
				V := normalize(reflection)
				intensity += light.intensity * math.Pow(math.Max(0, dot(R, V)), specular)
			}
		}
	}

	return intensity
}

func CanvasToViewPort(x int, y int) Vector {
	return MakeVector(float64(x)*Vw/Cw, float64(y)*Vh/Ch, d)
}
