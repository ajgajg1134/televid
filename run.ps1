param(
    [string]$File
)

go run main.go $File
ffmpeg -framerate 5 -i imgs/%04d.png -c:v prores -pix_fmt yuva444p10le output.mov