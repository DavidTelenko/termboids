// Boid compute shader
// Calculates boid forces and updates positions/velocities on GPU

struct Boid {
    pos_x: f32,
    pos_y: f32,
    vel_x: f32,
    vel_y: f32,
}

struct Config {
    max_speed: f32,
    max_force: f32,
    separation_radius: f32,
    alignment_radius: f32,
    cohesion_radius: f32,
    separation_weight: f32,
    alignment_weight: f32,
    cohesion_weight: f32,
    random_weight: f32,
    width: f32,
    height: f32,
    delta_time: f32,
    num_boids: u32,
    frame_count: u32, // For randomness
    // Repellant config
    repellant_active: u32,  // 0 = inactive, 1 = active
    repellant_x: f32,
    repellant_y: f32,
    repellant_radius: f32,
    repellant_strength: f32,
    // Attractor config
    attractor_active: u32,  // 0 = inactive, 1 = active
    attractor_x: f32,
    attractor_y: f32,
    attractor_radius: f32,
    attractor_strength: f32,
    _padding: u32,  // Align to 16 bytes
}

@group(0) @binding(0) var<storage, read> boids_in: array<Boid>;
@group(0) @binding(1) var<storage, read_write> boids_out: array<Boid>;
@group(0) @binding(2) var<uniform> config: Config;

// Simple hash function for random numbers
fn hash(x: u32) -> f32 {
    var h = x;
    h = h ^ (h >> 16u);
    h = h * 0x85ebca6bu;
    h = h ^ (h >> 13u);
    h = h * 0xc2b2ae35u;
    h = h ^ (h >> 16u);
    return f32(h) / 4294967295.0;
}

// Random value between -1 and 1
fn random_11(seed: u32) -> f32 {
    return hash(seed) * 2.0 - 1.0;
}

// Vector operations
fn length_vec2(v: vec2<f32>) -> f32 {
    return sqrt(v.x * v.x + v.y * v.y);
}

fn normalize_vec2(v: vec2<f32>) -> vec2<f32> {
    let len = length_vec2(v);
    if (len > 0.0) {
        return v / len;
    }
    return vec2<f32>(0.0, 0.0);
}

fn limit_vec2(v: vec2<f32>, max_len: f32) -> vec2<f32> {
    let len = length_vec2(v);
    if (len > max_len) {
        return (v / len) * max_len;
    }
    return v;
}

fn distance_squared(p1: vec2<f32>, p2: vec2<f32>) -> f32 {
    let dx = p1.x - p2.x;
    let dy = p1.y - p2.y;
    return dx * dx + dy * dy;
}

// Wrap position to screen bounds (toroidal topology)
fn wrap_position(pos: vec2<f32>) -> vec2<f32> {
    var result = pos;

    if (result.x < 0.0) {
        result.x = result.x + config.width;
    } else if (result.x >= config.width) {
        result.x = result.x - config.width;
    }

    if (result.y < 0.0) {
        result.y = result.y + config.height;
    } else if (result.y >= config.height) {
        result.y = result.y - config.height;
    }

    return result;
}

// Calculate shortest distance considering wrapping
fn wrapped_difference(p1: vec2<f32>, p2: vec2<f32>) -> vec2<f32> {
    var diff = p1 - p2;

    // Handle X wrapping
    if (abs(diff.x) > config.width * 0.5) {
        if (diff.x > 0.0) {
            diff.x = diff.x - config.width;
        } else {
            diff.x = diff.x + config.width;
        }
    }

    // Handle Y wrapping
    if (abs(diff.y) > config.height * 0.5) {
        if (diff.y > 0.0) {
            diff.y = diff.y - config.height;
        } else {
            diff.y = diff.y + config.height;
        }
    }

    return diff;
}

@compute @workgroup_size(64)
fn main(@builtin(global_invocation_id) global_id: vec3<u32>) {
    let index = global_id.x;

    // Bounds check
    if (index >= config.num_boids) {
        return;
    }

    // Read current boid data
    let boid = boids_in[index];
    let pos = vec2<f32>(boid.pos_x, boid.pos_y);
    let vel = vec2<f32>(boid.vel_x, boid.vel_y);

    // Accumulators for flocking behaviors
    var separation = vec2<f32>(0.0, 0.0);
    var alignment = vec2<f32>(0.0, 0.0);
    var cohesion = vec2<f32>(0.0, 0.0);
    var separation_count = 0u;
    var alignment_count = 0u;
    var cohesion_count = 0u;

    // Pre-calculate squared radii
    let sep_radius_sq = config.separation_radius * config.separation_radius;
    let align_radius_sq = config.alignment_radius * config.alignment_radius;
    let coh_radius_sq = config.cohesion_radius * config.cohesion_radius;

    // Check all other boids
    for (var i = 0u; i < config.num_boids; i = i + 1u) {
        if (i == index) {
            continue;
        }

        let other = boids_in[i];
        let other_pos = vec2<f32>(other.pos_x, other.pos_y);
        let other_vel = vec2<f32>(other.vel_x, other.vel_y);

        // Calculate distance considering wrapping
        let diff = wrapped_difference(pos, other_pos);
        let dist_sq = diff.x * diff.x + diff.y * diff.y;
        let dist = sqrt(dist_sq);

        // Separation: steer away from nearby boids
        if (dist_sq < sep_radius_sq && dist > 0.0) {
            let weighted_diff = normalize_vec2(diff) * (1.0 / dist);
            separation = separation + weighted_diff;
            separation_count = separation_count + 1u;
        }

        // Alignment: match velocity of nearby boids
        if (dist_sq < align_radius_sq) {
            alignment = alignment + other_vel;
            alignment_count = alignment_count + 1u;
        }

        // Cohesion: move towards average position of nearby boids
        if (dist_sq < coh_radius_sq) {
            cohesion = cohesion + other_pos;
            cohesion_count = cohesion_count + 1u;
        }
    }

    // Calculate steering forces
    var acceleration = vec2<f32>(0.0, 0.0);

    // Repellant force (if active)
    if (config.repellant_active != 0u) {
        let repellant_pos = vec2<f32>(config.repellant_x, config.repellant_y);
        let diff = wrapped_difference(pos, repellant_pos);
        let dist = length_vec2(diff);
        
        if (dist < config.repellant_radius && dist > 0.0) {
            // Steer away from repellant point
            // Stronger force when closer
            let strength = (config.repellant_radius - dist) / config.repellant_radius;
            let repellant_force = normalize_vec2(diff) * config.max_force * strength * config.repellant_strength;
            acceleration = acceleration + repellant_force;
        }
    }

    // Attractor force (if active)
    if (config.attractor_active != 0u) {
        let attractor_pos = vec2<f32>(config.attractor_x, config.attractor_y);
        let diff = wrapped_difference(attractor_pos, pos);
        let dist = length_vec2(diff);
        
        if (dist < config.attractor_radius && dist > 0.0) {
            // Steer towards attractor point
            // Stronger force when closer
            let strength = (config.attractor_radius - dist) / config.attractor_radius;
            let attractor_force = normalize_vec2(diff) * config.max_force * strength * config.attractor_strength;
            acceleration = acceleration + attractor_force;
        }
    }

    // Separation
    if (separation_count > 0u) {
        separation = separation / f32(separation_count);
        if (length_vec2(separation) > 0.0) {
            separation = normalize_vec2(separation) * config.max_speed;
            separation = separation - vel;
            separation = limit_vec2(separation, config.max_force);
            acceleration = acceleration + separation * config.separation_weight;
        }
    }

    // Alignment
    if (alignment_count > 0u) {
        alignment = alignment / f32(alignment_count);
        alignment = normalize_vec2(alignment) * config.max_speed;
        alignment = alignment - vel;
        alignment = limit_vec2(alignment, config.max_force);
        acceleration = acceleration + alignment * config.alignment_weight;
    }

    // Cohesion
    if (cohesion_count > 0u) {
        cohesion = cohesion / f32(cohesion_count);
        var desired = wrapped_difference(cohesion, pos);
        desired = normalize_vec2(desired) * config.max_speed;
        desired = desired - vel;
        desired = limit_vec2(desired, config.max_force);
        acceleration = acceleration + desired * config.cohesion_weight;
    }

    // Add random force to prevent stabilization
    // Use boid index and frame count for deterministic randomness
    let rand_seed = index * 1000u + config.frame_count;
    let rand_chance = hash(rand_seed);
    if (config.random_weight > 0.0 && rand_chance < 0.1) {
        let rand_x = random_11(rand_seed + 1u);
        let rand_y = random_11(rand_seed + 2u);
        let random_force = normalize_vec2(vec2<f32>(rand_x, rand_y)) * config.max_force * config.random_weight;
        acceleration = acceleration + random_force;
    }

    // Update velocity and position
    let new_vel = limit_vec2(vel + acceleration * config.delta_time, config.max_speed);
    let new_pos = wrap_position(pos + new_vel * config.delta_time);

    // Write output
    boids_out[index].pos_x = new_pos.x;
    boids_out[index].pos_y = new_pos.y;
    boids_out[index].vel_x = new_vel.x;
    boids_out[index].vel_y = new_vel.y;
}
