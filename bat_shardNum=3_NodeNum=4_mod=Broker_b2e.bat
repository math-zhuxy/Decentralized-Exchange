rmdir /s /q bytecode
rmdir /s /q record
rmdir /s /q log
rmdir /s /q key
go build -o blockEmulator_Windows_Precompile.exe ./main.go

start cmd /k blockEmulator_Windows_Precompile.exe -n 1 -N 4 -s 0 -S 3 -m 4

start cmd /k blockEmulator_Windows_Precompile.exe -n 1 -N 4 -s 1 -S 3 -m 4

start cmd /k blockEmulator_Windows_Precompile.exe -n 2 -N 4 -s 0 -S 3 -m 4

start cmd /k blockEmulator_Windows_Precompile.exe -n 2 -N 4 -s 1 -S 3 -m 4

start cmd /k blockEmulator_Windows_Precompile.exe -n 3 -N 4 -s 0 -S 3 -m 4

start cmd /k blockEmulator_Windows_Precompile.exe -n 3 -N 4 -s 1 -S 3 -m 4





start cmd /k blockEmulator_Windows_Precompile.exe -n 1 -N 4 -s 2 -S 3 -m 4

start cmd /k blockEmulator_Windows_Precompile.exe -n 2 -N 4 -s 2 -S 3 -m 4

start cmd /k blockEmulator_Windows_Precompile.exe -n 3 -N 4 -s 2 -S 3 -m 4



start cmd /k blockEmulator_Windows_Precompile.exe -n 0 -N 4 -s 0 -S 3 -m 4
start cmd /k blockEmulator_Windows_Precompile.exe -n 0 -N 4 -s 1 -S 3 -m 4
start cmd /k blockEmulator_Windows_Precompile.exe -n 0 -N 4 -s 2 -S 3 -m 4

start cmd /k blockEmulator_Windows_Precompile.exe -N 4 -S 3 -m 4 -c