CREATE TABLE releases (
	name VARCHAR(100) PRIMARY KEY,
	version VARCHAR(20),
	binary_url VARCHAR(100),
	instructions TEXT
);

INSERT INTO releases (name, version, binary_url, instructions) VALUES ("project-aliaser","0.0.2","github/pa","");