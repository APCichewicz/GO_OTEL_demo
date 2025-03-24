-- name: GetAllUsers :many
select * from users;

-- name: GetUserByUsername :one
select * from users where name = $1;

-- name: InsertUser :one
insert into users (email, name, password) values ($1, $2, $3) returning *;

-- name: GetUserByEmail :one
select * from users where email = $1;
    
-- name: GetUserById :one
select * from users where id = $1;

-- name: UpdateUser :one
update users set email = $1, name = $2, password = $3 where id = $4 returning *;

-- name: DeleteUser :one
delete from users where id = $1 returning *;
