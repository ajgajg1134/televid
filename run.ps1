go run main.go
ffmpeg -framerate 5 -i imgs/%04d.png -c:v prores -pix_fmt yuva444p10le output.mov