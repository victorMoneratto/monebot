#!/usr/bin/env bash
go install && env $(cat .env | xargs) ${PWD##*/}
