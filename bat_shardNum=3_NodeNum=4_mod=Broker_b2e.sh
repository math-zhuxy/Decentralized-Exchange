rm -rf ./bytecode
rm -rf ./record
rm -rf ./log
rm -rf ./key

# go build -o blockEmulator_Linux_Precompile ./main.go

./blockEmulator_Linux_Precompile -n 1 -N 4 -s 0 -S 3 -m 4 &

./blockEmulator_Linux_Precompile -n 1 -N 4 -s 1 -S 3 -m 4 &

./blockEmulator_Linux_Precompile -n 2 -N 4 -s 0 -S 3 -m 4 &

./blockEmulator_Linux_Precompile -n 2 -N 4 -s 1 -S 3 -m 4 &

./blockEmulator_Linux_Precompile -n 3 -N 4 -s 0 -S 3 -m 4 &

./blockEmulator_Linux_Precompile -n 3 -N 4 -s 1 -S 3 -m 4 &





./blockEmulator_Linux_Precompile  -n 1 -N 4 -s 2 -S 3 -m 4 &

./blockEmulator_Linux_Precompile  -n 2 -N 4 -s 2 -S 3 -m 4 &

./blockEmulator_Linux_Precompile  -n 3 -N 4 -s 2 -S 3 -m 4 &



./blockEmulator_Linux_Precompile  -n 0 -N 4 -s 0 -S 3 -m 4 &
./blockEmulator_Linux_Precompile  -n 0 -N 4 -s 1 -S 3 -m 4 &
./blockEmulator_Linux_Precompile  -n 0 -N 4 -s 2 -S 3 -m 4 &

./blockEmulator_Linux_Precompile  -N 4 -S 3 -m 4 -c &