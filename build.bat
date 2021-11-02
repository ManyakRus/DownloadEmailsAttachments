set FileName=DownloadEmailsAttachments.exe

go build main.go
del /Q %FileName%
ren main.exe %FileName%
copy %FileName% DownloadEmailsAttachments_ready\%FileName%
copy readme.txt DownloadEmailsAttachments_ready\readme.txt
copy readme.md DownloadEmailsAttachments_ready\readme.md
