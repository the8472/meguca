select insert_post(
	$1::bool,
	$2::bool,
	$3::bigint,
	$4::varchar(3),
	$5::bigint,
	$6::bigint,
	$7::varchar(2000),
	$8::varchar(50),
	$9::char(10),
	$10::varchar(20),
	$11::bytea,
	$12::inet,
	$13::char(40),
	$14::varchar(200),
	$15::bigint[][2],
	$16::bigint[][2],
	$17::json[]
);
